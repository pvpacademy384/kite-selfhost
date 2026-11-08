package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kitecloud/kite/kite-service/pkg/thing"
)

const (
	tursoIntegrationID = "turso"
	tursoMaxSQLLength  = 4000
	tursoMaxParams     = 20
	// Rows past this are left out of the result, which flows keep in memory.
	tursoMaxRows       = 100
	tursoRequestTimeout = 15 * time.Second
)

var (
	tursoFirstWordRe = regexp.MustCompile(`^[A-Za-z]+`)
	tursoWhereRe     = regexp.MustCompile(`(?i)\bwhere\b`)
)

func init() {
	registerHandlers(map[FlowNodeType]nodeHandler{
		FlowNodeTypeActionTursoRead:  executeActionTursoRead,
		FlowNodeTypeActionTursoWrite: executeActionTursoWrite,
		FlowNodeTypeActionTursoCheck: executeActionTursoCheck,
	})
}

// ParseTursoCredential splits the credential an app enters for Turso, the
// database URL and the auth token with a "|" between them, into the address
// requests go to and the token. The address has to be a Turso one, as the app
// chooses it, so a credential can't point requests at another server. The
// errors never contain the credential.
func ParseTursoCredential(credential string) (string, string, error) {
	rawURL, token, ok := strings.Cut(strings.TrimSpace(credential), "|")
	rawURL = strings.TrimSpace(rawURL)
	token = strings.TrimSpace(token)
	if !ok || rawURL == "" || token == "" {
		return "", "", errors.New("enter the database URL, a | and the auth token, like libsql://my-db-me.turso.io|eyJ...")
	}

	if strings.HasPrefix(rawURL, "libsql://") {
		rawURL = "https://" + strings.TrimPrefix(rawURL, "libsql://")
	}

	u, err := url.Parse(rawURL)
	if err != nil ||
		u.Scheme != "https" ||
		u.User != nil ||
		u.Port() != "" ||
		(u.Path != "" && u.Path != "/") ||
		u.RawQuery != "" ||
		u.Fragment != "" {
		return "", "", errors.New("the database URL must look like libsql://my-db-me.turso.io")
	}

	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, ".turso.io") || len(host) <= len(".turso.io") {
		return "", "", errors.New("the database URL must be a Turso address ending in .turso.io")
	}

	return "https://" + host, token, nil
}

func executeActionTursoRead(n *CompiledFlowNode, ctx *FlowContext) error {
	sql, err := tursoSQL(n)
	if err != nil {
		return err
	}

	switch strings.ToLower(tursoFirstWordRe.FindString(sql)) {
	case "select", "with", "values", "explain":
	default:
		return errors.New("Read rows runs SELECT queries. Use Write data to change data")
	}

	args, err := tursoArgs(n, ctx)
	if err != nil {
		return err
	}

	res, err := tursoRun(ctx, sql, args)
	if err != nil {
		return err
	}

	columns := make([]thing.Thing, len(res.Cols))
	for i, col := range res.Cols {
		columns[i] = thing.NewString(col.Name)
	}

	rows := make([]thing.Thing, 0, min(len(res.Rows), tursoMaxRows))
	for i, row := range res.Rows {
		if i >= tursoMaxRows {
			break
		}
		obj := make(map[string]thing.Thing, len(row))
		for j, value := range row {
			name := "column_" + strconv.Itoa(j)
			if j < len(res.Cols) {
				name = res.Cols[j].Name
			}
			obj[name] = value.toThing()
		}
		rows = append(rows, thing.NewObject(obj))
	}

	first := thing.Null
	if len(rows) > 0 {
		first = rows[0]
	}

	ctx.StoreNodeResult(n, thing.NewObject(map[string]thing.Thing{
		"rows":      thing.NewArray(rows),
		"columns":   thing.NewArray(columns),
		"first":     first,
		"count":     thing.NewInt(len(rows)),
		"found":     thing.NewBool(len(rows) > 0),
		"truncated": thing.NewBool(len(res.Rows) > tursoMaxRows),
	}))
	return n.ExecuteChildren(ctx)
}

func executeActionTursoWrite(n *CompiledFlowNode, ctx *FlowContext) error {
	sql, err := tursoSQL(n)
	if err != nil {
		return err
	}

	word := strings.ToLower(tursoFirstWordRe.FindString(sql))
	switch word {
	case "insert", "update", "delete", "replace", "create", "alter", "with":
	default:
		return errors.New("Write data runs INSERT, UPDATE, DELETE, REPLACE, CREATE and ALTER. Use Read rows for SELECT. DROP isn't allowed in flows, do it in the Turso dashboard")
	}
	// One block shouldn't be able to change every row by forgetting a WHERE.
	if (word == "update" || word == "delete") && !tursoWhereRe.MatchString(sql) {
		return errors.New("UPDATE and DELETE need a WHERE, so one block can't change every row. Use WHERE 1 = 1 if you mean all rows")
	}

	args, err := tursoArgs(n, ctx)
	if err != nil {
		return err
	}

	res, err := tursoRun(ctx, sql, args)
	if err != nil {
		return err
	}

	lastInsertID := thing.Null
	if id, err := strconv.ParseInt(strings.Trim(string(res.LastInsertRowID), `"`), 10, 64); err == nil {
		lastInsertID = thing.NewInt(id)
	}

	ctx.StoreNodeResult(n, thing.NewObject(map[string]thing.Thing{
		"affected_rows":  thing.NewInt(res.AffectedRowCount),
		"last_insert_id": lastInsertID,
	}))
	return n.ExecuteChildren(ctx)
}

func executeActionTursoCheck(n *CompiledFlowNode, ctx *FlowContext) error {
	start := time.Now()
	if _, err := tursoRun(ctx, "SELECT 1", nil); err != nil {
		return err
	}

	ctx.StoreNodeResult(n, thing.NewObject(map[string]thing.Thing{
		"ok":         thing.NewBool(true),
		"latency_ms": thing.NewInt(time.Since(start).Milliseconds()),
	}))
	return n.ExecuteChildren(ctx)
}

func tursoSetting(n *CompiledFlowNode, name string) string {
	value, _ := n.Data.Setting(name).(string)
	return value
}

// tursoSQL returns the query of a block. It's used as it is and never
// evaluated as a template, so what members type into a command can't change
// the query, only the values after it.
func tursoSQL(n *CompiledFlowNode) (string, error) {
	sql := strings.TrimSpace(tursoSetting(n, "turso_sql"))
	sql = strings.TrimSpace(strings.TrimSuffix(sql, ";"))
	switch {
	case sql == "":
		return "", errors.New("the SQL query is empty")
	case len(sql) > tursoMaxSQLLength:
		return "", fmt.Errorf("the SQL query can be at most %d characters", tursoMaxSQLLength)
	case strings.Contains(sql, "{{"):
		return "", errors.New("placeholders don't work in the SQL query. Put a ? where the value goes and the placeholder in the values setting")
	case strings.Contains(sql, ";"):
		return "", errors.New("a block runs one statement. Use a second block for the next one, and ? for values that contain a ;")
	}
	return sql, nil
}

// tursoArgs returns the values for the ? marks of the query, one per line of
// the setting. Each line is evaluated on its own, so a value with line breaks
// stays one value. Whole numbers are sent as numbers and everything else as
// text, which SQLite converts for the column it's compared with or stored in.
func tursoArgs(n *CompiledFlowNode, ctx *FlowContext) ([]tursoArg, error) {
	raw := strings.TrimSpace(tursoSetting(n, "turso_params"))
	if raw == "" {
		return nil, nil
	}

	lines := strings.Split(raw, "\n")
	if len(lines) > tursoMaxParams {
		return nil, fmt.Errorf("a query can have at most %d values", tursoMaxParams)
	}

	args := make([]tursoArg, 0, len(lines))
	for _, line := range lines {
		value, err := ctx.EvalTemplate(strings.TrimRight(line, "\r"))
		if err != nil {
			return nil, err
		}

		s := value.String()
		if i, err := strconv.ParseInt(s, 10, 64); err == nil && strconv.FormatInt(i, 10) == s {
			args = append(args, tursoArg{Type: "integer", Value: s})
		} else {
			args = append(args, tursoArg{Type: "text", Value: s})
		}
	}
	return args, nil
}

type tursoArg struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type tursoExecuteResult struct {
	Cols []struct {
		Name string `json:"name"`
	} `json:"cols"`
	Rows             [][]tursoValue  `json:"rows"`
	AffectedRowCount int64           `json:"affected_row_count"`
	LastInsertRowID  json.RawMessage `json:"last_insert_rowid"`
}

type tursoValue struct {
	Type   string          `json:"type"`
	Value  json.RawMessage `json:"value"`
	Base64 string          `json:"base64"`
}

func (v tursoValue) toThing() thing.Thing {
	switch v.Type {
	case "integer":
		var s string
		if err := json.Unmarshal(v.Value, &s); err == nil {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				return thing.NewInt(i)
			}
			return thing.NewString(s)
		}
	case "float":
		var f float64
		if err := json.Unmarshal(v.Value, &f); err == nil {
			return thing.NewFloat(f)
		}
	case "text":
		var s string
		if err := json.Unmarshal(v.Value, &s); err == nil {
			return thing.NewString(s)
		}
	case "blob":
		return thing.NewString(v.Base64)
	}
	return thing.Null
}

type tursoPipelineResponse struct {
	Results []struct {
		Type     string `json:"type"`
		Response *struct {
			Result *tursoExecuteResult `json:"result"`
		} `json:"response"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"results"`
}

// tursoRun runs one statement in the app's Turso database. The credential
// only goes to the Turso address it contains, and errors don't show it.
func tursoRun(ctx *FlowContext, sql string, args []tursoArg) (*tursoExecuteResult, error) {
	integration, ok := integrations[tursoIntegrationID]
	if !ok {
		return nil, errors.New("the Turso integration is missing")
	}

	credential, err := integrationCredential(ctx, integration)
	if err != nil {
		return nil, err
	}

	baseURL, token, err := ParseTursoCredential(credential)
	if err != nil {
		return nil, fmt.Errorf("the Turso credential is wrong, replace it in the app's integrations: %w", err)
	}
	redact := &flowSecrets{values: map[string]string{"credential": credential, "token": token}}

	stmt := map[string]any{"sql": sql}
	if len(args) > 0 {
		stmt["args"] = args
	}
	body, err := json.Marshal(map[string]any{
		"requests": []any{
			map[string]any{"type": "execute", "stmt": stmt},
			map[string]any{"type": "close"},
		},
	})
	if err != nil {
		return nil, err
	}

	reqCtx, cancel := context.WithTimeout(ctx, tursoRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/v2/pipeline", bytes.NewReader(body))
	if err != nil {
		return nil, redact.Redact(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := ctx.HTTP.HTTPRequestWithoutRedirects(reqCtx, req)
	if err != nil {
		return nil, redact.Redact(err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, thing.MaxBodySize+1))
	if err != nil {
		return nil, redact.Redact(err)
	}
	if len(data) > thing.MaxBodySize {
		return nil, fmt.Errorf("the result is bigger than %d bytes, select fewer rows or columns", thing.MaxBodySize)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := []rune(redact.redactString(string(data)))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		hint := ""
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			hint = " (check the auth token)"
		}
		return nil, fmt.Errorf("%s returned %s%s: %s", integration.Name, resp.Status, hint, string(msg))
	}

	var pipeline tursoPipelineResponse
	if err := json.Unmarshal(data, &pipeline); err != nil {
		return nil, fmt.Errorf("failed to parse the response of %s: %w", integration.Name, err)
	}
	if len(pipeline.Results) == 0 {
		return nil, fmt.Errorf("%s returned no result", integration.Name)
	}

	first := pipeline.Results[0]
	if first.Type == "error" || first.Error != nil {
		message := "unknown error"
		if first.Error != nil {
			message = first.Error.Message
		}
		return nil, redact.Redact(fmt.Errorf("%s rejected the query: %s", integration.Name, message))
	}
	if first.Response == nil || first.Response.Result == nil {
		return nil, fmt.Errorf("%s returned no result", integration.Name)
	}
	return first.Response.Result, nil
}

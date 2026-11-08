import { Integration } from "../types";

// The address of a Turso database is different for every app, so it's part of
// the credential, the database URL and the auth token with a "|" between them,
// instead of a base_url. The service only accepts Turso addresses.
export const turso: Integration = {
  id: "turso",
  name: "Turso",
  description:
    "Read and write the data in your own Turso database, for things like points, warnings or settings that need more than stored variables.",
  auth: {
    type: "header",
    name: "Authorization",
    prefix: "Bearer ",
    label: "database URL and token",
    help_url: "https://docs.turso.tech/",
  },
  availability: "opt_in",
  data_shared:
    "Enter your database URL, then a | and your auth token, like libsql://my-db-me.turso.io|eyJhbGci... Its blocks send Turso the queries and values you set, and read back the rows. A token that can only read, made with turso db tokens create my-db --read-only, is the safest choice if you only read.",
};

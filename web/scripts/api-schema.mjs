import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import openapiTS, { astToString } from "openapi-typescript";
const schema = JSON.parse(
  execFileSync("go", ["run", "../internal/apischema"], { encoding: "utf8" }),
);
const generated =
  "// Generated from the Go HTTP contract. Run npm run generate:api.\n" +
  astToString(await openapiTS(schema));
const target = "src/api-schema.d.ts";
if (process.argv.includes("--check")) {
  if (readFileSync(target, "utf8") !== generated)
    throw new Error("API types have drifted. Run npm run generate:api.");
} else writeFileSync(target, generated);

import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { format } from "prettier";
import openapiTS, { astToString } from "openapi-typescript";
for (const client of [false, true]) {
  const schema = JSON.parse(
    execFileSync(
      "go",
      ["run", "../internal/apischema", ...(client ? ["--client"] : [])],
      { encoding: "utf8" },
    ),
  );
  const generated =
    "// Generated from the Go HTTP contract. Run npm run generate:api.\n" +
    astToString(await openapiTS(schema));
  const target = client ? "src/client-api-schema.d.ts" : "src/api-schema.d.ts";
  if (process.argv.includes("--check")) {
    if (readFileSync(target, "utf8") !== generated)
      throw new Error("API types have drifted. Run npm run generate:api.");
  } else writeFileSync(target, generated);
}

const profiles = JSON.parse(
  execFileSync("go", ["run", "../internal/apischema", "--preview-profiles"], {
    encoding: "utf8",
  }),
);
const generatedProfiles = await format(
  "// Generated from internal/derive.PreviewProfiles. Run npm run generate:api.\nexport const mediaProfiles = " +
    JSON.stringify(profiles) +
    " as const;\n",
  { parser: "typescript" },
);
const profilesTarget = "src/media-profiles.generated.ts";
if (process.argv.includes("--check")) {
  if (readFileSync(profilesTarget, "utf8") !== generatedProfiles)
    throw new Error("Preview profiles have drifted. Run npm run generate:api.");
} else writeFileSync(profilesTarget, generatedProfiles);

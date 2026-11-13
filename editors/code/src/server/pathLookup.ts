import { execFileSync } from "node:child_process";

type LookupCommand = (
  command: string,
  args: string[],
  options: { encoding: "utf-8"; stdio: "pipe" }
) => string;

const runLookup: LookupCommand = (command, args, options) => execFileSync(command, args, options);

export function findExecutableOnPath(
  name: string,
  platform: NodeJS.Platform = process.platform,
  run: LookupCommand = runLookup
): string | undefined {
  try {
    const output = run(platform === "win32" ? "where" : "which", [name], {
      encoding: "utf-8",
      stdio: "pipe",
    });
    return output.split(/\r?\n/u).find((line) => line.length > 0);
  } catch {
    return undefined;
  }
}

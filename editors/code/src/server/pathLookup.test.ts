import { describe, expect, it, vi } from "vitest";
import { findExecutableOnPath } from "./pathLookup";

describe("findExecutableOnPath", () => {
  it("uses where.exe on Windows and takes the first result", () => {
    const run = vi.fn().mockReturnValue("C:\\tools\\skyls.exe\r\nC:\\other\\skyls.exe\r\n");
    expect(findExecutableOnPath("skyls", "win32", run)).toBe("C:\\tools\\skyls.exe");
    expect(run).toHaveBeenCalledWith("where", ["skyls"], {
      encoding: "utf-8",
      stdio: "pipe",
    });
  });

  it("uses which on Unix and returns undefined when missing", () => {
    const run = vi.fn().mockImplementation(() => {
      throw new Error("not found");
    });
    expect(findExecutableOnPath("skyls", "linux", run)).toBeUndefined();
    expect(run).toHaveBeenCalledWith("which", ["skyls"], {
      encoding: "utf-8",
      stdio: "pipe",
    });
  });
});

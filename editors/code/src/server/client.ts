import { execSync } from "node:child_process";
import * as vscode from "vscode";
import {
  LanguageClient,
  type LanguageClientOptions,
  type ServerOptions,
  State,
  TransportKind,
} from "vscode-languageclient/node";
import type { ExtensionContext } from "../context";

const SERVER_BINARY_NAME = "skyls";

export async function activateLanguageServer(ctx: ExtensionContext): Promise<void> {
  ctx.setStatus("starting");

  const serverPath = findServerBinary(ctx);
  if (serverPath === undefined) {
    ctx.setStatus("error", "Language server binary not found");
    return;
  }

  const serverOptions: ServerOptions = {
    run: {
      command: serverPath,
      args: [],
      transport: TransportKind.stdio,
    },
    debug: {
      command: serverPath,
      args: ["-v"],
      transport: TransportKind.stdio,
    },
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: "file", language: "starlark" },
      { scheme: "untitled", language: "starlark" },
    ],
    outputChannel: ctx.outputChannel,
    traceOutputChannel: ctx.outputChannel,
  };

  ctx.client = new LanguageClient(
    "sky-starlark",
    "Sky - Starlark Language Server",
    serverOptions,
    clientOptions
  );

  ctx.client.onDidChangeState((event) => {
    switch (event.newState) {
      case State.Stopped:
        ctx.setStatus("stopped");
        break;
      case State.Starting:
        ctx.setStatus("starting");
        break;
      case State.Running:
        ctx.setStatus("ready");
        break;
    }
  });

  await ctx.client.start();
}

function findServerBinary(ctx: ExtensionContext): string | undefined {
  const config = vscode.workspace.getConfiguration("sky-starlark");
  const explicitPath = config.get<string>("server.path");

  // 1. Check explicit configuration
  if (explicitPath !== undefined && explicitPath !== "") {
    ctx.outputChannel.info(`Using configured server path: ${explicitPath}`);
    return explicitPath;
  }

  // 2. Check if server is in PATH
  const which = findInPath(SERVER_BINARY_NAME);
  if (which !== undefined) {
    ctx.outputChannel.info(`Found ${SERVER_BINARY_NAME} in PATH: ${which}`);
    return which;
  }

  // 3. TODO: Check bundled binary in extension directory
  // const bundled = getBundledServerPath(ctx);
  // if (bundled !== undefined) return bundled;

  ctx.outputChannel.warn(
    `Language server '${SERVER_BINARY_NAME}' not found. ` +
      `Please install it or set 'sky-starlark.server.path' in settings.`
  );
  return undefined;
}

/**
 * Simple PATH lookup for the server binary.
 * Returns the full path if found, undefined otherwise.
 */
function findInPath(name: string): string | undefined {
  try {
    const result = execSync(`which ${name}`, { encoding: "utf-8" }).trim();
    return result !== "" ? result : undefined;
  } catch {
    return undefined;
  }
}

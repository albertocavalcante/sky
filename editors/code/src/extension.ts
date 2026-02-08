import * as vscode from "vscode";
import { ExtensionContext } from "./context";
import { activateLanguageServer } from "./server/client";

let ctx: ExtensionContext | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const outputChannel = vscode.window.createOutputChannel("Sky - Starlark", { log: true });
  outputChannel.info("Activating Sky - Starlark extension...");

  const extCtx = new ExtensionContext(context, outputChannel);
  ctx = extCtx;

  await activateLanguageServer(extCtx);

  extCtx.registerCommand("sky-starlark.restartServer", async () => {
    outputChannel.info("Restarting language server...");
    if (extCtx.client !== undefined) {
      await extCtx.client.stop();
    }
    await activateLanguageServer(extCtx);
  });

  outputChannel.info("Sky - Starlark extension activated");
}

export function deactivate(): Promise<void> | undefined {
  return ctx?.dispose();
}

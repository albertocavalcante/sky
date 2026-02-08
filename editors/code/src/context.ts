import * as vscode from "vscode";
import type { LanguageClient } from "vscode-languageclient/node";

type ServerStatus = "starting" | "ready" | "error" | "stopped";

/**
 * Central extension context managing all state and providing clean disposal.
 * Pattern derived from rust-analyzer.
 */
export class ExtensionContext {
  client: LanguageClient | undefined;
  serverVersion: string | undefined;

  private readonly disposables: vscode.Disposable[] = [];
  private readonly statusBarItem: vscode.StatusBarItem;

  constructor(
    readonly extensionContext: vscode.ExtensionContext,
    readonly outputChannel: vscode.LogOutputChannel
  ) {
    this.statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
    this.statusBarItem.name = "Sky - Starlark";
    this.disposables.push(this.statusBarItem);
  }

  registerCommand(command: string, callback: (...args: unknown[]) => Promise<void> | void): void {
    const disposable = vscode.commands.registerCommand(command, callback);
    this.extensionContext.subscriptions.push(disposable);
  }

  setStatus(status: ServerStatus, detail?: string): void {
    const name = "Sky - Starlark";
    switch (status) {
      case "starting":
        this.statusBarItem.text = `$(sync~spin) ${name}`;
        this.statusBarItem.tooltip = `${name}: Starting...`;
        break;
      case "ready":
        this.statusBarItem.text = `$(check) ${name}`;
        this.statusBarItem.tooltip = detail !== undefined ? `${name}: ${detail}` : `${name}: Ready`;
        break;
      case "error":
        this.statusBarItem.text = `$(error) ${name}`;
        this.statusBarItem.tooltip = detail !== undefined ? `${name}: ${detail}` : `${name}: Error`;
        break;
      case "stopped":
        this.statusBarItem.text = `$(circle-slash) ${name}`;
        this.statusBarItem.tooltip = `${name}: Stopped`;
        break;
    }
    this.statusBarItem.show();
  }

  async dispose(): Promise<void> {
    if (this.client !== undefined) {
      await this.client.stop();
    }
    for (const d of this.disposables) {
      d.dispose();
    }
  }
}

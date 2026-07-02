import { exec } from "node:child_process";

export function runUserCommand(cmd: string): void {
  exec(cmd);
}

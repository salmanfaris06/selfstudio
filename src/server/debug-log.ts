import fs from 'node:fs/promises';
import path from 'node:path';
import { inspect } from 'node:util';

export type DebugLogLevel = 'debug' | 'info' | 'warn' | 'error';

export type DebugLogEntry = {
  timestamp: string;
  level: DebugLogLevel;
  area: string;
  message: string;
  data?: unknown;
};

const logDir = path.join(process.cwd(), 'data', 'logs');
export const debugLogPath = path.join(logDir, 'debug-events.jsonl');

export async function logDebugEvent(entry: Omit<DebugLogEntry, 'timestamp'>): Promise<void> {
  const fullEntry: DebugLogEntry = {
    timestamp: new Date().toISOString(),
    ...entry
  };

  const line = `${JSON.stringify(sanitizeForJson(fullEntry))}\n`;
  await fs.mkdir(logDir, { recursive: true });
  await fs.appendFile(debugLogPath, line, 'utf8');

  const consoleLine = `[${fullEntry.timestamp}] [${fullEntry.level.toUpperCase()}] [${fullEntry.area}] ${fullEntry.message}`;
  if (fullEntry.level === 'error') {
    console.error(consoleLine, fullEntry.data ?? '');
  } else if (fullEntry.level === 'warn') {
    console.warn(consoleLine, fullEntry.data ?? '');
  } else {
    console.log(consoleLine, fullEntry.data ?? '');
  }
}

export async function readDebugEvents(limit = 200): Promise<DebugLogEntry[]> {
  const safeLimit = Number.isInteger(limit) && limit > 0 ? Math.min(limit, 1000) : 200;

  try {
    const content = await fs.readFile(debugLogPath, 'utf8');
    return content
      .split(/\r?\n/)
      .filter(Boolean)
      .slice(-safeLimit)
      .flatMap((line) => {
        try {
          return [JSON.parse(line) as DebugLogEntry];
        } catch {
          return [];
        }
      });
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
      return [];
    }
    throw error;
  }
}

export function serializeError(error: unknown): Record<string, unknown> {
  if (error instanceof Error) {
    return {
      name: error.name,
      message: error.message,
      stack: error.stack
    };
  }
  return { value: String(error) };
}

function sanitizeForJson(value: unknown): unknown {
  return JSON.parse(
    JSON.stringify(value, (_key, currentValue) => {
      if (typeof currentValue === 'bigint') {
        return currentValue.toString();
      }
      if (currentValue instanceof Error) {
        return serializeError(currentValue);
      }
      if (typeof currentValue === 'function') {
        return `[Function ${currentValue.name || 'anonymous'}]`;
      }
      if (typeof currentValue === 'undefined') {
        return null;
      }
      return currentValue;
    })
  );
}

export function debugInspect(value: unknown): string {
  return inspect(value, { depth: 4, breakLength: 120 });
}

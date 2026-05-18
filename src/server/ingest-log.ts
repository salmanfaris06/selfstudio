import fs from 'node:fs/promises';
import path from 'node:path';

export type IngestEvent = {
  timestamp: string;
  stationId: string;
  sourcePath: string;
  filename: string;
  fileSize: number;
  modifiedTime: string;
  status: 'accepted';
};

const logDir = path.join(process.cwd(), 'data', 'logs');
export const ingestLogPath = path.join(logDir, 'ingest-events.jsonl');

export async function appendIngestEvent(event: IngestEvent): Promise<void> {
  await fs.mkdir(logDir, { recursive: true });
  await fs.appendFile(ingestLogPath, `${JSON.stringify(event)}\n`, 'utf8');
}

export async function readIngestEvents(limit = 100): Promise<IngestEvent[]> {
  const safeLimit = clampEventLimit(limit);

  try {
    const content = await fs.readFile(ingestLogPath, 'utf8');
    return content
      .split(/\r?\n/)
      .filter(Boolean)
      .slice(-safeLimit)
      .flatMap((line) => {
        try {
          return [JSON.parse(line) as IngestEvent];
        } catch {
          return [];
        }
      });
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === 'ENOENT') {
      return [];
    }
    throw error;
  }
}

function clampEventLimit(limit: number): number {
  if (!Number.isInteger(limit) || limit < 1) {
    return 100;
  }
  return Math.min(limit, 500);
}

import { getApiBaseUrl } from "@/lib/api/client";
import type { EventEnvelope } from "@/lib/events/types";

export function createEventStream() {
  if (typeof EventSource === "undefined") {
    throw new Error("EventSource is only available in the browser.");
  }

  return new EventSource(`${getApiBaseUrl()}/events`, { withCredentials: true });
}

export function parseEventEnvelope<TData extends Record<string, unknown> = Record<string, unknown>>(
  message: MessageEvent<string>,
): EventEnvelope<TData> {
  const parsed = JSON.parse(message.data) as Partial<EventEnvelope<TData>>;
  if (!isEventEnvelope<TData>(parsed)) {
    throw new Error("Invalid Selfstudio event envelope.");
  }

  return parsed;
}

function isEventEnvelope<TData extends Record<string, unknown>>(
  value: Partial<EventEnvelope<TData>>,
): value is EventEnvelope<TData> {
  return (
    typeof value.event_id === "string" &&
    typeof value.event_type === "string" &&
    typeof value.entity_type === "string" &&
    typeof value.entity_id === "string" &&
    typeof value.occurred_at === "string" &&
    typeof value.data === "object" &&
    value.data !== null &&
    !Array.isArray(value.data)
  );
}

export type EventEnvelope<TData extends Record<string, unknown> = Record<string, unknown>> = {
  event_id: string;
  event_type: string;
  entity_type: string;
  entity_id: string;
  occurred_at: string;
  data: TData;
};

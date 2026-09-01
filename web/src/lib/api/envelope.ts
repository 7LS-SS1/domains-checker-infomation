import { z } from "zod";

/**
 * Mirrors the Go API's envelope exactly: internal/api/response.go writes
 * either {"data": ...} or {"error": {...}}. Every field here is load-bearing
 * against the running backend contract documented in
 * web/API_CONTRACT_MATRIX.md section 10 and internal/i18n/i18n.go.
 */
export const localeSchema = z.union([z.literal("th"), z.literal("en")]);
export type Locale = z.infer<typeof localeSchema>;

export const apiErrorPayloadSchema = z.object({
  code: z.string().min(1),
  message: z.string(),
  messages: z.object({
    th: z.string(),
    en: z.string(),
  }),
  locale: localeSchema,
  request_id: z.string(),
  details: z.record(z.string(), z.unknown()).optional(),
});

export type ApiErrorPayload = z.infer<typeof apiErrorPayloadSchema>;

const apiErrorEnvelopeSchema = z.object({
  error: apiErrorPayloadSchema,
});

const apiSuccessEnvelopeSchema = z.object({
  data: z.unknown(),
});

/**
 * Thrown for every non-2xx backend/BFF response. Callers branch on `code`
 * (e.g. "UNAUTHORIZED", "FORBIDDEN", "CSRF_INVALID") per
 * web/FRONTEND_ARCHITECTURE.md section 1.5 — never on `message`, which is
 * locale text, not a stable identifier.
 */
export class ApiError extends Error {
  readonly code: string;
  readonly requestId: string;
  readonly locale: Locale;
  readonly messages: { th: string; en: string };
  readonly details: Record<string, unknown> | undefined;
  readonly status: number;

  constructor(payload: ApiErrorPayload, status: number) {
    super(payload.message);
    this.name = "ApiError";
    this.code = payload.code;
    this.requestId = payload.request_id;
    this.locale = payload.locale;
    this.messages = payload.messages;
    this.details = payload.details;
    this.status = status;
  }
}

/**
 * Raised when the backend/BFF returned a response that is not JSON, or is
 * JSON but matches neither the success nor the error envelope shape. This is
 * distinct from ApiError: it means the contract itself was violated, not
 * that the requested operation failed cleanly.
 */
export class MalformedApiResponseError extends Error {
  readonly status: number;
  constructor(status: number, cause?: unknown) {
    super("The API response did not match the expected envelope shape.");
    this.name = "MalformedApiResponseError";
    this.status = status;
    this.cause = cause;
  }
}

/**
 * Parses a fetch Response against the {data}/{error} envelope contract and
 * either returns the unwrapped `data` (typed via the caller-supplied
 * dataSchema) or throws ApiError / MalformedApiResponseError. Never returns
 * a value for a non-2xx response — mirrors backend behavior where status
 * code and envelope shape always agree (writeData vs writeError).
 */
export async function parseApiEnvelope<Schema extends z.ZodTypeAny>(
  response: Response,
  dataSchema: Schema,
): Promise<z.infer<Schema>> {
  // 204 No Content (e.g. DELETE /google-drive/connection,
  // internal/api/phase5.go disconnectGoogleDrive) has no body to parse —
  // treat it as a successful empty result rather than a JSON parse failure.
  if (response.status === 204) {
    const emptyResult = dataSchema.safeParse(undefined);
    if (!emptyResult.success) {
      throw new MalformedApiResponseError(response.status, emptyResult.error);
    }
    return emptyResult.data;
  }

  let json: unknown;
  try {
    json = await response.json();
  } catch (cause) {
    throw new MalformedApiResponseError(response.status, cause);
  }

  if (!response.ok) {
    const parsedError = apiErrorEnvelopeSchema.safeParse(json);
    if (!parsedError.success) {
      throw new MalformedApiResponseError(response.status, parsedError.error);
    }
    throw new ApiError(parsedError.data.error, response.status);
  }

  const parsedSuccess = apiSuccessEnvelopeSchema.safeParse(json);
  if (!parsedSuccess.success) {
    throw new MalformedApiResponseError(response.status, parsedSuccess.error);
  }

  const dataResult = dataSchema.safeParse(parsedSuccess.data.data);
  if (!dataResult.success) {
    throw new MalformedApiResponseError(response.status, dataResult.error);
  }
  return dataResult.data;
}

/**
 * Selects the localized message from an already-fetched error payload
 * without requiring a re-fetch. Used when the UI's active locale changes
 * client-side after an error was captured, or when rendering the same error
 * in a locale-aware component. Falls back to the request-time `message`
 * (the locale the backend selected via Accept-Language) if the desired
 * locale is somehow missing from `messages` — this should never happen
 * given the schema above requires both th and en, but stays defensive.
 */
export function selectErrorMessage(
  payload: Pick<ApiErrorPayload, "message" | "messages">,
  locale: Locale,
): string {
  return payload.messages[locale] || payload.message;
}

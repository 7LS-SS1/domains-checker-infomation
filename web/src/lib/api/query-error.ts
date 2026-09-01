import { ApiError } from "@/lib/api/envelope";

export interface QueryErrorInfo {
  message?: string;
  requestId?: string;
}

/** Extracts a display-ready message/request ID from a TanStack Query error, if any. */
export function describeQueryError(error: unknown): QueryErrorInfo {
  if (error instanceof ApiError) {
    return { message: error.message, requestId: error.requestId };
  }
  return {};
}

"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useLocale } from "next-intl";
import { ApiError } from "@/lib/api/envelope";
import { bffFetch } from "@/lib/api/client";
import { useConnectDrive } from "@/lib/drive/queries";
import { driveConnectionSchema } from "@/lib/drive/types";

const POLL_INTERVAL_MS = 2_000;
const MAX_POLL_ATTEMPTS = 60; // ~120s bounded consent window

export type DriveOAuthState = "idle" | "connecting" | "popup-blocked" | "error";

/**
 * Implements the popup + bounded-poll + opener-closes-popup OAuth lifecycle
 * from CLAUDE_FRONTEND_PROMPTS.md Prompt 6. The popup is opened
 * synchronously (blank, then redirected once the real authorization_url is
 * fetched) so the click's "trusted user gesture" isn't lost to the async
 * request — most popup blockers only allow window.open() called directly
 * within the gesture handler.
 */
export function useDriveOAuthPopup(onConnected: () => void) {
  const [state, setState] = useState<DriveOAuthState>("idle");
  const [pollAttempts, setPollAttempts] = useState(0);
  const popupRef = useRef<Window | null>(null);
  const connectMutation = useConnectDrive();
  const queryClient = useQueryClient();
  const locale = useLocale();

  const stopPolling = useCallback(() => {
    popupRef.current = null;
    setPollAttempts(0);
  }, []);

  useEffect(() => {
    if (state !== "connecting") {
      return;
    }
    const timer = setTimeout(async () => {
      if (!popupRef.current || popupRef.current.closed) {
        stopPolling();
        setState("idle");
        return;
      }
      if (pollAttempts >= MAX_POLL_ATTEMPTS) {
        popupRef.current.close();
        stopPolling();
        setState("idle");
        return;
      }
      const result = await queryClient
        .fetchQuery({
          queryKey: ["drive-connection", "poll"],
          queryFn: () =>
            bffFetch("/api/bff/google-drive/connection", driveConnectionSchema, { locale }),
          retry: false,
          staleTime: 0,
        })
        .catch(() => null);

      if (result?.status === "active") {
        popupRef.current.close();
        stopPolling();
        setState("idle");
        void queryClient.invalidateQueries({ queryKey: ["drive-connection"] });
        onConnected();
        return;
      }
      setPollAttempts((count) => count + 1);
    }, POLL_INTERVAL_MS);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- onConnected/queryClient are stable enough for this bounded poll loop
  }, [state, pollAttempts, stopPolling]);

  const connect = useCallback(() => {
    setState("connecting");
    const popup = window.open("", "google-drive-oauth", "width=520,height=650");
    if (!popup) {
      setState("popup-blocked");
      return;
    }
    popupRef.current = popup;

    connectMutation.mutate(undefined, {
      onSuccess: (authorization) => {
        if (popupRef.current && !popupRef.current.closed) {
          popupRef.current.location.href = authorization.authorization_url;
        }
      },
      onError: () => {
        popupRef.current?.close();
        stopPolling();
        setState("error");
      },
    });
  }, [connectMutation, stopPolling]);

  const errorMessage =
    connectMutation.error instanceof ApiError ? connectMutation.error.message : undefined;

  return { state, connect, errorMessage };
}

"use client";

import { Toaster } from "sonner";
import { useTheme } from "next-themes";

/**
 * sonner renders its region with role="status"/aria-live built in, so
 * toasts are announced to assistive tech without extra wiring here.
 */
export function ToastProvider() {
  const { resolvedTheme } = useTheme();
  return (
    <Toaster
      theme={resolvedTheme === "dark" ? "dark" : "light"}
      position="top-right"
      richColors
      closeButton
    />
  );
}

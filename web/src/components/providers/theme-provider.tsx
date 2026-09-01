"use client";

import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ReactNode } from "react";

/**
 * next-themes toggles a "dark" class on <html> (see the `dark` custom
 * variant wired in src/app/globals.css). enableSystem + defaultTheme
 * "system" means the theme respects the OS preference until the user
 * explicitly picks one; the choice then persists in localStorage
 * (non-sensitive UI preference, not session data).
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  return (
    <NextThemesProvider attribute="class" defaultTheme="system" enableSystem>
      {children}
    </NextThemesProvider>
  );
}

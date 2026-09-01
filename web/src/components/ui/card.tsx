import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils/cn";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("rounded-lg border border-border bg-background shadow-sm", className)}
      {...props}
    />
  );
}

export function CardHeader({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      // flex-wrap: a title plus one or more action buttons must wrap to a
      // second line at narrow viewports rather than force page-level
      // horizontal overflow — found via a real WCAG/responsive check
      // failing on ProbesPageContent's header at 900px width, but this is
      // the shared primitive every CardHeader in the app renders through.
      className={cn("flex flex-wrap items-center justify-between gap-2 p-4 pb-2", className)}
      {...props}
    />
  );
}

export function CardTitle({ className, ...props }: HTMLAttributes<HTMLHeadingElement>) {
  return <h2 className={cn("text-sm font-semibold text-foreground", className)} {...props} />;
}

export function CardContent({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn("p-4 pt-2", className)} {...props} />;
}

import { AlertTriangle } from "lucide-react";
import { useTranslations } from "next-intl";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils/cn";

interface ErrorStateProps {
  title: string;
  description?: string;
  requestId?: string;
  onRetry?: () => void;
  className?: string;
}

/**
 * Generic inline error state for a data-fetching panel — distinct from
 * EmptyState (which means "the request succeeded and there is genuinely
 * nothing"). Never collapse the two: an error must never render as if it
 * were a legitimate zero-result state.
 */
export function ErrorState({ title, description, requestId, onRetry, className }: ErrorStateProps) {
  const t = useTranslations("common");

  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center gap-2 rounded-lg border border-status-rose-border bg-status-rose-bg p-6 text-center",
        className,
      )}
    >
      <AlertTriangle size={24} className="text-status-rose-fg" aria-hidden />
      <p className="text-sm font-medium text-status-rose-fg">{title}</p>
      {description && <p className="max-w-md text-sm text-status-rose-fg/80">{description}</p>}
      {requestId && (
        <p className="text-xs text-status-rose-fg/70">
          {t("requestId")}: {requestId}
        </p>
      )}
      {onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry} className="mt-1">
          {t("retry")}
        </Button>
      )}
    </div>
  );
}

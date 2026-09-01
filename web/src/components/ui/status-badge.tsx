import type { LucideIcon } from "lucide-react";
import { STATUS_TONE_CLASSES, type StatusTone } from "@/lib/status/tokens";
import { cn } from "@/lib/utils/cn";

interface StatusBadgeProps {
  tone: StatusTone;
  icon: LucideIcon;
  label: string;
  className?: string;
}

/**
 * Icon + color + text, always together — never color alone, per WCAG 2.2 AA
 * and the Master Prompt UX rule. Domain-specific enum→tone/icon mapping
 * lives next to each feature (e.g. src/lib/incidents/status.ts); this
 * component only renders whatever it's given.
 */
export function StatusBadge({ tone, icon: Icon, label, className }: StatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium",
        STATUS_TONE_CLASSES[tone],
        className,
      )}
    >
      <Icon size={14} aria-hidden />
      <span>{label}</span>
    </span>
  );
}

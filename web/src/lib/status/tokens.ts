/**
 * The five status color families from web/FRONTEND_ARCHITECTURE.md
 * section 7 / the Master Prompt UX direction. Every domain-specific status
 * mapping (incidents, recommendations, domain health, probes, ...) resolves
 * to one of these — never an ad hoc sixth color.
 */
export type StatusTone = "emerald" | "amber" | "rose" | "slate" | "violet";

export const STATUS_TONE_CLASSES: Record<StatusTone, string> = {
  emerald: "bg-status-emerald-bg text-status-emerald-fg border-status-emerald-border",
  amber: "bg-status-amber-bg text-status-amber-fg border-status-amber-border",
  rose: "bg-status-rose-bg text-status-rose-fg border-status-rose-border",
  slate: "bg-status-slate-bg text-status-slate-fg border-status-slate-border",
  violet: "bg-status-violet-bg text-status-violet-fg border-status-violet-border",
};

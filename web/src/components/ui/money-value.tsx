import { formatDecimalString } from "@/lib/utils/decimal-format";
import { cn } from "@/lib/utils/cn";

interface MoneyValueProps {
  /** Decimal string from the backend (finance.Decimal / report money fields) — never a number. */
  amount: string;
  currency: string;
  className?: string;
}

/**
 * Renders a backend decimal string with thousands separators via pure
 * string manipulation (see formatDecimalString) — the amount is never
 * routed through Number()/parseFloat, per the Master Prompt's decimal
 * data-correctness rule.
 */
export function MoneyValue({ amount, currency, className }: MoneyValueProps) {
  return (
    <span className={cn("tabular-nums", className)}>
      {formatDecimalString(amount)} {currency}
    </span>
  );
}

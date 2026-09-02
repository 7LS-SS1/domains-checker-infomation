import { formatDecimalString } from "@/lib/utils/decimal-format";
import { cn } from "@/lib/utils/cn";

interface MoneyValueProps {
  /** Decimal string from the backend (finance.Decimal / report money fields) — never a number. */
  amount: string;
  currency: string;
  className?: string;
  /** Round display to this many fraction digits (via BigInt, not Number()/parseFloat). Defaults to 2, the standard currency display precision. */
  decimals?: number;
}

/**
 * Renders a backend decimal string with thousands separators via pure
 * string manipulation (see formatDecimalString) — the amount is never
 * routed through Number()/parseFloat, per the Master Prompt's decimal
 * data-correctness rule. Every currency amount in the app renders through
 * this component so the 2-decimal display precision stays consistent.
 */
export function MoneyValue({ amount, currency, className, decimals = 2 }: MoneyValueProps) {
  return (
    <span className={cn("tabular-nums", className)}>
      {formatDecimalString(amount, { decimals })} {currency}
    </span>
  );
}

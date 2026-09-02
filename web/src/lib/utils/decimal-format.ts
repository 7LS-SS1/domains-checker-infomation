/**
 * Adds thousands separators to a decimal string for display, without ever
 * parsing it through Number()/parseFloat — pure string/regex manipulation
 * only, so arbitrarily precise or large decimal strings from the backend
 * (internal/finance/decimal.go uses big.Rat, not float64) are never at risk
 * of binary-floating-point rounding. The fractional part is passed through
 * byte-for-byte.
 */
export function formatDecimalString(raw: string, options?: { decimals?: number }): string {
  const trimmed = raw.trim();
  const negative = trimmed.startsWith("-");
  const unsigned = negative ? trimmed.slice(1) : trimmed;
  const [integerPart = "0", decimalPart] = unsigned.split(".");

  let finalInteger = integerPart;
  let finalDecimal = decimalPart;

  if (options?.decimals !== undefined) {
    const decimals = options.decimals;
    const digits = decimalPart ?? "";
    if (digits.length <= decimals) {
      finalDecimal = decimals > 0 ? digits.padEnd(decimals, "0") : undefined;
    } else {
      // Round via BigInt on the digit string (never Number()/parseFloat) so
      // arbitrarily large/precise backend decimals still round exactly.
      const kept = digits.slice(0, decimals);
      const roundUp = digits.charAt(decimals) >= "5";
      const combined = BigInt(integerPart + kept) + (roundUp ? 1n : 0n);
      const combinedStr = combined.toString().padStart(decimals + 1, "0");
      finalInteger = decimals > 0 ? combinedStr.slice(0, -decimals) || "0" : combinedStr;
      finalDecimal = decimals > 0 ? combinedStr.slice(-decimals) : undefined;
    }
  }

  const grouped = finalInteger.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const result = finalDecimal !== undefined ? `${grouped}.${finalDecimal}` : grouped;
  return negative ? `-${result}` : result;
}

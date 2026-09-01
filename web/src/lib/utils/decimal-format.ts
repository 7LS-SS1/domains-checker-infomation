/**
 * Adds thousands separators to a decimal string for display, without ever
 * parsing it through Number()/parseFloat — pure string/regex manipulation
 * only, so arbitrarily precise or large decimal strings from the backend
 * (internal/finance/decimal.go uses big.Rat, not float64) are never at risk
 * of binary-floating-point rounding. The fractional part is passed through
 * byte-for-byte.
 */
export function formatDecimalString(raw: string): string {
  const trimmed = raw.trim();
  const negative = trimmed.startsWith("-");
  const unsigned = negative ? trimmed.slice(1) : trimmed;
  const [integerPart = "0", decimalPart] = unsigned.split(".");
  const grouped = integerPart.replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const result = decimalPart !== undefined ? `${grouped}.${decimalPart}` : grouped;
  return negative ? `-${result}` : result;
}

import { describe, expect, it } from "vitest";
import en from "@/messages/en.json";
import th from "@/messages/th.json";

/**
 * Flattens a nested message catalog into dotted leaf-key paths, e.g.
 * {"sync":{"drive":{"connect":"..."}}} -> ["sync.drive.connect"]. Used to
 * diff the th/en catalogs structurally rather than just checking both files
 * parse — a key present in one locale but missing in the other means a
 * literal key string (or a blank string) would render on screen, which the
 * Master Prompt's i18n catalog discipline forbids.
 */
function flattenKeys(value: unknown, prefix = ""): string[] {
  if (typeof value === "string") {
    return [prefix];
  }
  if (value && typeof value === "object") {
    return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
      flattenKeys(child, prefix ? `${prefix}.${key}` : key),
    );
  }
  return [prefix];
}

describe("th/en message catalog parity", () => {
  const enKeys = new Set(flattenKeys(en));
  const thKeys = new Set(flattenKeys(th));

  it("every English key has a Thai counterpart", () => {
    const missingInThai = [...enKeys].filter((key) => !thKeys.has(key));
    expect(missingInThai).toEqual([]);
  });

  it("every Thai key has an English counterpart", () => {
    const missingInEnglish = [...thKeys].filter((key) => !enKeys.has(key));
    expect(missingInEnglish).toEqual([]);
  });

  it("no message value is an empty string in either locale", () => {
    function emptyLeaves(value: unknown, prefix = ""): string[] {
      if (typeof value === "string") {
        return value.trim() === "" ? [prefix] : [];
      }
      if (value && typeof value === "object") {
        return Object.entries(value as Record<string, unknown>).flatMap(([key, child]) =>
          emptyLeaves(child, prefix ? `${prefix}.${key}` : key),
        );
      }
      return [];
    }
    expect(emptyLeaves(en)).toEqual([]);
    expect(emptyLeaves(th)).toEqual([]);
  });
});

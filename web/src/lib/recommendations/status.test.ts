import { describe, expect, it } from "vitest";
import {
  recommendationActionIcon,
  recommendationActionTone,
  RECOMMENDATION_ACTIONS,
} from "@/lib/recommendations/status";

describe("recommendation action → tone/icon mapping", () => {
  it.each(RECOMMENDATION_ACTIONS)("maps every known action %s to a tone and an icon", (action) => {
    expect(recommendationActionTone(action)).toBeTruthy();
    expect(recommendationActionIcon(action)).toBeTruthy();
  });

  it("never maps two different actions to a tone that collides with an unrelated meaning", () => {
    expect(recommendationActionTone("RENEW")).toBe("emerald");
    expect(recommendationActionTone("REVIEW")).toBe("amber");
    expect(recommendationActionTone("DROP")).toBe("rose");
    expect(recommendationActionTone("PROFIT_OPPORTUNITY")).toBe("violet");
  });

  it("falls back to slate for an unrecognized action rather than crashing", () => {
    expect(recommendationActionTone("SOMETHING_NEW")).toBe("slate");
    expect(recommendationActionIcon("SOMETHING_NEW")).toBeTruthy();
  });
});

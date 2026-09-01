import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { DomainsFilterBar } from "@/components/domains/domains-filter-bar";
import enMessages from "@/messages/en.json";

function renderFilterBar(overrides: Partial<React.ComponentProps<typeof DomainsFilterBar>> = {}) {
  const onQueryChange = vi.fn();
  const onLifecycleChange = vi.fn();
  const onSourceChange = vi.fn();
  render(
    <NextIntlClientProvider locale="en" messages={enMessages}>
      <DomainsFilterBar
        query=""
        lifecycleStatus=""
        sourceStatus=""
        onQueryChange={onQueryChange}
        onLifecycleChange={onLifecycleChange}
        onSourceChange={onSourceChange}
        {...overrides}
      />
    </NextIntlClientProvider>,
  );
  return { onQueryChange, onLifecycleChange, onSourceChange };
}

describe("DomainsFilterBar", () => {
  it("types into the search box without dropping or reordering characters", async () => {
    const user = userEvent.setup();
    renderFilterBar();
    const input = screen.getByPlaceholderText("Search domain...");
    await user.type(input, "example.com");
    expect(input).toHaveValue("example.com");
  });

  it("debounces the query change callback", async () => {
    const user = userEvent.setup();
    const { onQueryChange } = renderFilterBar();
    const input = screen.getByPlaceholderText("Search domain...");
    await user.type(input, "test");
    expect(onQueryChange).not.toHaveBeenCalled();
    await waitFor(() => expect(onQueryChange).toHaveBeenCalledWith("test"), { timeout: 1000 });
  });

  it("fires lifecycle/source filter changes immediately (not debounced)", async () => {
    const user = userEvent.setup();
    const { onLifecycleChange } = renderFilterBar();
    await user.selectOptions(screen.getByLabelText("Lifecycle status"), "active");
    expect(onLifecycleChange).toHaveBeenCalledWith("active");
  });
});

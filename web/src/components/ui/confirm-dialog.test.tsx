import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

function renderDialog(onConfirm = vi.fn()) {
  render(
    <ConfirmDialog
      open={true}
      onOpenChange={() => {}}
      title="Archive this domain?"
      description="This can be reversed."
      reasonLabel="Reason"
      confirmLabel="Confirm"
      cancelLabel="Cancel"
      onConfirm={onConfirm}
    />,
  );
  return onConfirm;
}

describe("ConfirmDialog (mandatory reason validation)", () => {
  it("keeps the confirm button disabled until a reason is entered", async () => {
    renderDialog();
    const confirmButton = screen.getByRole("button", { name: "Confirm" });
    expect(confirmButton).toBeDisabled();

    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Reason"), "Domain retired, replaced by new brand");
    expect(confirmButton).toBeEnabled();
  });

  it("disables confirm again if the reason is cleared back to whitespace-only", async () => {
    renderDialog();
    const user = userEvent.setup();
    const textarea = screen.getByLabelText("Reason");
    await user.type(textarea, "a reason");
    expect(screen.getByRole("button", { name: "Confirm" })).toBeEnabled();

    await user.clear(textarea);
    await user.type(textarea, "   ");
    expect(screen.getByRole("button", { name: "Confirm" })).toBeDisabled();
  });

  it("calls onConfirm with the trimmed reason when submitted", async () => {
    const onConfirm = renderDialog();
    const user = userEvent.setup();
    await user.type(screen.getByLabelText("Reason"), "  cleanup unused domain  ");
    await user.click(screen.getByRole("button", { name: "Confirm" }));
    expect(onConfirm).toHaveBeenCalledWith("cleanup unused domain");
  });

  it("never calls onConfirm while the reason is empty, even via Enter/submit", async () => {
    const onConfirm = renderDialog();
    const form = screen.getByRole("button", { name: "Confirm" }).closest("form")!;
    form.requestSubmit();
    expect(onConfirm).not.toHaveBeenCalled();
  });
});

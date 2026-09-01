"use client";

import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/envelope";
import { useAddDomainCost } from "@/lib/finance/mutations";
import { COST_TYPES, TAX_MODES } from "@/lib/finance/types";

const DECIMAL_PATTERN = /^[0-9]+(?:\.[0-9]{1,6})?$/;

export function AddCostDialog({
  open,
  onOpenChange,
  domainId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  domainId: string;
}) {
  const t = useTranslations("domainDetail.finance.form");
  const tCommon = useTranslations("common");
  const addCost = useAddDomainCost();

  const schema = z.object({
    cost_type: z.enum(COST_TYPES),
    amount: z.string().regex(DECIMAL_PATTERN, "Must be a plain decimal amount"),
    currency: z
      .string()
      .length(3)
      .transform((value) => value.toUpperCase()),
    tax_rate: z.string().optional(),
    tax_mode: z.enum(TAX_MODES),
    billing_cycle_months: z.string().refine((value) => {
      const parsed = Number(value);
      return Number.isInteger(parsed) && parsed >= 1 && parsed <= 120;
    }, "Must be an integer between 1 and 120"),
    effective_from: z.string().optional(),
    source_reference: z.string().optional(),
    reason: z.string().min(1),
  });
  type FormValues = z.infer<typeof schema>;

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      cost_type: "renewal",
      amount: "",
      currency: "THB",
      tax_rate: "",
      tax_mode: "unknown",
      billing_cycle_months: "12",
      effective_from: "",
      source_reference: "",
      reason: "",
    },
  });

  const onSubmit = handleSubmit((values) => {
    addCost.mutate(
      {
        domainId,
        cost_type: values.cost_type,
        amount: values.amount,
        currency: values.currency,
        tax_rate: values.tax_rate ? values.tax_rate : null,
        tax_mode: values.tax_mode,
        billing_cycle_months: Number(values.billing_cycle_months),
        effective_from: values.effective_from || undefined,
        source_reference: values.source_reference,
        reason: values.reason,
      },
      { onSuccess: () => onOpenChange(false) },
    );
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={tCommon("cancel")}>
        <DialogTitle>{t("title")}</DialogTitle>
        <form onSubmit={onSubmit} noValidate className="mt-4 space-y-4">
          {addCost.isError && (
            <p role="alert" className="text-sm text-status-rose-fg">
              {addCost.error instanceof ApiError ? addCost.error.message : tCommon("loadError")}
            </p>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="cost_type">{t("costType")}</Label>
              <select
                id="cost_type"
                className="h-10 w-full rounded-md border border-border bg-background px-2 text-sm"
                {...register("cost_type")}
              >
                {COST_TYPES.map((type) => (
                  <option key={type} value={type}>
                    {type}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="tax_mode">{t("taxMode")}</Label>
              <select
                id="tax_mode"
                className="h-10 w-full rounded-md border border-border bg-background px-2 text-sm"
                {...register("tax_mode")}
              >
                {TAX_MODES.map((mode) => (
                  <option key={mode} value={mode}>
                    {mode}
                  </option>
                ))}
              </select>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="amount">{t("amount")}</Label>
              <Input id="amount" inputMode="decimal" placeholder="0.00" {...register("amount")} />
              {errors.amount && (
                <p role="alert" className="text-sm text-status-rose-fg">
                  {errors.amount.message}
                </p>
              )}
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="currency">{t("currency")}</Label>
              <Input id="currency" maxLength={3} {...register("currency")} />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="tax_rate">{t("taxRate")}</Label>
              <Input
                id="tax_rate"
                inputMode="decimal"
                placeholder="0.07"
                {...register("tax_rate")}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="billing_cycle_months">{t("billingCycleMonths")}</Label>
              <Input
                id="billing_cycle_months"
                type="number"
                min={1}
                max={120}
                {...register("billing_cycle_months")}
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="effective_from">{t("effectiveFrom")}</Label>
            <Input id="effective_from" type="date" {...register("effective_from")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="source_reference">{t("sourceReference")}</Label>
            <Input id="source_reference" {...register("source_reference")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="cost_reason">{tCommon("reason")}</Label>
            <Input
              id="cost_reason"
              placeholder={tCommon("reasonPlaceholder")}
              {...register("reason")}
            />
            {errors.reason && (
              <p role="alert" className="text-sm text-status-rose-fg">
                {errors.reason.message}
              </p>
            )}
          </div>

          <div className="flex justify-end gap-2">
            <Button type="submit" disabled={addCost.isPending}>
              {addCost.isPending && <Loader2 className="animate-spin" size={16} aria-hidden />}
              {t("submit")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

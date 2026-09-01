"use client";

import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useTranslations } from "next-intl";
import { Loader2 } from "lucide-react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ApiError } from "@/lib/api/envelope";
import { useAddExchangeRate } from "@/lib/finance/exchange-rate-mutations";

const RATE_PATTERN = /^[0-9]+(?:\.[0-9]{1,10})?$/;

export function AddExchangeRateDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useTranslations("financePage.form");
  const tCommon = useTranslations("common");
  const addRate = useAddExchangeRate();

  const schema = z.object({
    base_currency: z
      .string()
      .length(3)
      .transform((v) => v.toUpperCase()),
    quote_currency: z
      .string()
      .length(3)
      .transform((v) => v.toUpperCase()),
    rate: z.string().regex(RATE_PATTERN, "Must be a positive decimal"),
    source: z.string().min(1),
    observed_at: z.string().min(1),
    reason: z.string().min(1),
  });
  type FormValues = z.input<typeof schema>;

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      base_currency: "USD",
      quote_currency: "THB",
      rate: "",
      source: "",
      observed_at: new Date().toISOString().slice(0, 16),
      reason: "",
    },
  });

  const onSubmit = handleSubmit((values) => {
    addRate.mutate(
      {
        base_currency: values.base_currency,
        quote_currency: values.quote_currency,
        rate: values.rate,
        source: values.source,
        observed_at: new Date(values.observed_at).toISOString(),
        reason: values.reason,
      },
      { onSuccess: () => onOpenChange(false) },
    );
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={tCommon("cancel")}>
        <DialogTitle>{t("title")}</DialogTitle>
        <DialogDescription>{t("description")}</DialogDescription>
        <form onSubmit={onSubmit} noValidate className="mt-4 space-y-4">
          {addRate.isError && (
            <p role="alert" className="text-sm text-status-rose-fg">
              {addRate.error instanceof ApiError ? addRate.error.message : tCommon("loadError")}
            </p>
          )}

          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label htmlFor="base_currency">{t("baseCurrency")}</Label>
              <Input id="base_currency" maxLength={3} {...register("base_currency")} />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="quote_currency">{t("quoteCurrency")}</Label>
              <Input id="quote_currency" maxLength={3} {...register("quote_currency")} />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="rate">{t("rate")}</Label>
            <Input id="rate" inputMode="decimal" {...register("rate")} />
            {errors.rate && (
              <p role="alert" className="text-sm text-status-rose-fg">
                {errors.rate.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="source">{t("source")}</Label>
            <Input id="source" {...register("source")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="observed_at">{t("observedAt")}</Label>
            <Input id="observed_at" type="datetime-local" {...register("observed_at")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="rate_reason">{tCommon("reason")}</Label>
            <Input
              id="rate_reason"
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
            <Button type="submit" disabled={addRate.isPending}>
              {addRate.isPending && <Loader2 className="animate-spin" size={16} aria-hidden />}
              {t("submit")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

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
import { useCreateOverride } from "@/lib/finance/mutations";
import { OVERRIDE_FIELDS } from "@/lib/finance/types";

export function CreateOverrideDialog({
  open,
  onOpenChange,
  domainId,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  domainId: string;
}) {
  const t = useTranslations("domainDetail.overrides.form");
  const tCommon = useTranslations("common");
  const createOverride = useCreateOverride();

  const schema = z.object({
    field_name: z.enum(OVERRIDE_FIELDS),
    override_value: z.string().min(1),
    expires_at: z.string().optional(),
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
      field_name: "business_priority",
      override_value: "",
      expires_at: "",
      reason: "",
    },
  });

  const onSubmit = handleSubmit((values) => {
    createOverride.mutate(
      {
        domainId,
        field_name: values.field_name,
        override_value: values.override_value,
        reason: values.reason,
        expires_at: values.expires_at ? new Date(values.expires_at).toISOString() : null,
      },
      { onSuccess: () => onOpenChange(false) },
    );
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent closeLabel={tCommon("cancel")}>
        <DialogTitle>{t("title")}</DialogTitle>
        <form onSubmit={onSubmit} noValidate className="mt-4 space-y-4">
          {createOverride.isError && (
            <p role="alert" className="text-sm text-status-rose-fg">
              {createOverride.error instanceof ApiError
                ? createOverride.error.message
                : tCommon("loadError")}
            </p>
          )}

          <div className="space-y-1.5">
            <Label htmlFor="field_name">{t("field")}</Label>
            <select
              id="field_name"
              className="h-10 w-full rounded-md border border-border bg-background px-2 text-sm"
              {...register("field_name")}
            >
              {OVERRIDE_FIELDS.map((field) => (
                <option key={field} value={field}>
                  {field}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="override_value">{t("value")}</Label>
            <Input id="override_value" {...register("override_value")} />
            {errors.override_value && (
              <p role="alert" className="text-sm text-status-rose-fg">
                {errors.override_value.message}
              </p>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="expires_at">{t("expiresAt")}</Label>
            <Input id="expires_at" type="date" {...register("expires_at")} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="override_reason">{tCommon("reason")}</Label>
            <Input
              id="override_reason"
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
            <Button type="submit" disabled={createOverride.isPending}>
              {createOverride.isPending && (
                <Loader2 className="animate-spin" size={16} aria-hidden />
              )}
              {t("submit")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

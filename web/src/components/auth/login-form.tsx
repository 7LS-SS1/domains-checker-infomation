"use client";

import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { z } from "zod";
import { useMutation } from "@tanstack/react-query";
import { useTranslations, useLocale } from "next-intl";
import { useRouter } from "next/navigation";
import { Eye, EyeOff, Loader2 } from "lucide-react";
import { bffFetch } from "@/lib/api/client";
import { loginDataSchema } from "@/lib/auth/types";
import { ApiError } from "@/lib/api/envelope";
import { sanitizeReturnTo } from "@/lib/auth/return-to";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

interface LoginFormProps {
  /** Already validated server-side by the /login page before being passed down. */
  returnTo: string;
}

export function LoginForm({ returnTo }: LoginFormProps) {
  const t = useTranslations("auth");
  const tCommon = useTranslations("common");
  const locale = useLocale();
  const router = useRouter();
  const [showPassword, setShowPassword] = useState(false);
  const [serverError, setServerError] = useState<{ message: string; requestId?: string } | null>(
    null,
  );

  const schema = z.object({
    email: z.string().min(1, t("emailRequired")).email(t("emailInvalid")),
    password: z.string().min(1, t("passwordRequired")),
  });
  type FormValues = z.infer<typeof schema>;

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    mode: "onBlur",
  });

  const mutation = useMutation({
    mutationFn: (values: FormValues) =>
      bffFetch("/api/bff/auth/login", loginDataSchema, {
        method: "POST",
        body: JSON.stringify(values),
        locale,
      }),
    onSuccess: () => {
      const target = sanitizeReturnTo(returnTo);
      router.replace(target);
      router.refresh();
    },
    onError: (error: unknown) => {
      if (error instanceof ApiError) {
        setServerError({ message: error.message, requestId: error.requestId });
      } else {
        setServerError({ message: t("genericError") });
      }
    },
  });

  const onSubmit = handleSubmit((values) => {
    setServerError(null);
    mutation.mutate(values);
  });

  return (
    <form onSubmit={onSubmit} noValidate className="space-y-4">
      {serverError && (
        <div
          id="login-error"
          role="alert"
          className="rounded-md border border-status-rose-border bg-status-rose-bg px-4 py-3 text-sm text-status-rose-fg"
        >
          <p>{serverError.message}</p>
          {serverError.requestId && (
            <p className="mt-1 text-xs opacity-80">
              {tCommon("requestId")}: {serverError.requestId}
            </p>
          )}
        </div>
      )}

      <div className="space-y-1.5">
        <Label htmlFor="email">{t("emailLabel")}</Label>
        <Input
          id="email"
          type="email"
          autoComplete="username"
          aria-invalid={!!errors.email}
          aria-describedby={errors.email ? "email-error" : undefined}
          {...register("email")}
        />
        {errors.email && (
          <p id="email-error" role="alert" className="text-sm text-status-rose-fg">
            {errors.email.message}
          </p>
        )}
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="password">{t("passwordLabel")}</Label>
        <div className="relative">
          <Input
            id="password"
            type={showPassword ? "text" : "password"}
            autoComplete="current-password"
            className="pr-11"
            aria-invalid={!!errors.password}
            aria-describedby={errors.password ? "password-error" : undefined}
            {...register("password")}
          />
          {/* h-10/w-10 matches the input's own height so the touch target
              is a full 40x40px square flush against the input's edge — the
              icon-sized-only version of this button failed axe's WCAG 2.5.8
              target-size check (18x40px, and insufficient space to the
              input's own hit area). */}
          <button
            type="button"
            onClick={() => setShowPassword((value) => !value)}
            aria-label={showPassword ? t("hidePassword") : t("showPassword")}
            aria-pressed={showPassword}
            className="absolute inset-y-0 right-0 flex h-10 w-10 items-center justify-center text-muted-foreground hover:text-foreground"
          >
            {showPassword ? <EyeOff size={18} aria-hidden /> : <Eye size={18} aria-hidden />}
          </button>
        </div>
        {errors.password && (
          <p id="password-error" role="alert" className="text-sm text-status-rose-fg">
            {errors.password.message}
          </p>
        )}
      </div>

      <Button type="submit" disabled={mutation.isPending} className="w-full">
        {mutation.isPending ? (
          <>
            <Loader2 className="animate-spin" size={16} aria-hidden />
            {t("submitting")}
          </>
        ) : (
          t("submit")
        )}
      </Button>
    </form>
  );
}

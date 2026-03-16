"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useRef, useState } from "react";

type ExchangeState = "working" | "success" | "error";

function parseErrorMessage(raw: string, status: number): string {
  if (!raw) {
    return `Exchange failed with HTTP ${status}`;
  }
  try {
    const parsed = JSON.parse(raw) as { error?: unknown; message?: unknown };
    if (typeof parsed.error === "string" && parsed.error) {
      return parsed.error;
    }
    if (typeof parsed.message === "string" && parsed.message) {
      return parsed.message;
    }
  } catch {
    return raw;
  }
  return `Exchange failed with HTTP ${status}`;
}

export function MagicLinkCallbackClient() {
  const router = useRouter();
  const params = useSearchParams();
  const [state, setState] = useState<ExchangeState>("working");
  const [message, setMessage] = useState("Verifying magic link...");
  const hasStarted = useRef(false);
  const magicLinkID = params.get("magic_link_id")?.trim() ?? "";
  const code = params.get("code")?.trim() ?? "";

  useEffect(() => {
    if (hasStarted.current) {
      return;
    }
    hasStarted.current = true;

    if (!magicLinkID || !code) {
      setState("error");
      setMessage("Missing magic_link_id or code in URL.");
      return;
    }

    let cancelled = false;
    const controller = new AbortController();
    const timeoutID = window.setTimeout(() => {
      controller.abort();
    }, 15000);

    const run = async () => {
      try {
        const response = await fetch("/v1/control/auth/magic-link/exchange", {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          signal: controller.signal,
          body: JSON.stringify({
            magic_link_id: magicLinkID,
            code
          })
        });
        const bodyText = await response.text();
        if (!response.ok) {
          throw new Error(parseErrorMessage(bodyText, response.status));
        }

        if (cancelled) {
          return;
        }
        setState("success");
        setMessage("Login successful. Redirecting to console...");

        router.replace("/base/console");
      } catch (error) {
        if (cancelled) {
          return;
        }
        if (error instanceof Error && error.name === "AbortError") {
          setState("error");
          setMessage("Magic link exchange timed out. Please open the link again.");
          return;
        }
        setState("error");
        setMessage(error instanceof Error ? error.message : "Magic link exchange failed.");
      } finally {
        window.clearTimeout(timeoutID);
      }
    };

    void run();

    return () => {
      cancelled = true;
      hasStarted.current = false;
      controller.abort();
      window.clearTimeout(timeoutID);
    };
  }, [code, magicLinkID, router]);

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-xl flex-col items-center justify-center px-6 text-center">
      <h1 className="mb-3 text-2xl font-semibold">Magic Link Sign-In</h1>
      <p className="mb-8 text-sm text-neutral-500">{message}</p>

      {state === "working" ? (
        <div className="text-sm text-neutral-400">Processing...</div>
      ) : null}

      {state === "success" ? (
        <div className="rounded-lg border border-emerald-300 bg-emerald-50 p-4 text-left text-sm text-emerald-900">
          <p>Session established.</p>
        </div>
      ) : null}

      {state === "error" ? (
        <div className="space-y-3">
          <div className="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-900">
            {message}
          </div>
          <div className="flex items-center justify-center gap-3">
            <Link className="text-sm underline" href="/">
              Back to home
            </Link>
            <Link className="text-sm underline" href="/base/console">
              Open base console
            </Link>
          </div>
        </div>
      ) : null}
    </main>
  );
}

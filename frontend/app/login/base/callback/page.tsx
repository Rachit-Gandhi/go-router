import { Suspense } from "react";

import { MagicLinkCallbackClient } from "@/app/login/callback/callback-client";

export default function LegacyMagicLinkCallbackPage() {
  return (
    <Suspense
      fallback={
        <main className="mx-auto flex min-h-screen w-full max-w-xl flex-col items-center justify-center px-6 text-center">
          <h1 className="mb-3 text-2xl font-semibold">Magic Link Sign-In</h1>
          <p className="text-sm text-neutral-500">Loading...</p>
        </main>
      }
    >
      <MagicLinkCallbackClient />
    </Suspense>
  );
}

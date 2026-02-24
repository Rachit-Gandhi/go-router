"use client";

import { useState } from "react";
import { AuthDialog } from "@/components/AuthDialog";
import { Header } from "@/components/Header";
import { Hero } from "@/components/Hero";
import { StatsGrid } from "@/components/StatsGrid";

const cards = [
  {
    title: "One API for Any Model",
    text: "Access all major models through one unified interface and OpenAI SDK compatibility.",
    link: "Browse all",
  },
  {
    title: "Higher Availability",
    text: "Route across providers with resilient fallbacks to reduce downtime.",
    link: "Learn more",
  },
  {
    title: "Price and Performance",
    text: "Keep latency low while balancing cost across providers.",
    link: "Learn more",
  },
  {
    title: "Custom Data Policies",
    text: "Choose trusted providers and enforce stricter privacy controls.",
    link: "View docs",
  },
];

export function LandingPage() {
  const [dialogOpen, setDialogOpen] = useState(false);

  return (
    <>
      <Header onSignupClick={() => setDialogOpen(true)} />
      <main className="pb-16">
        <Hero onSignupClick={() => setDialogOpen(true)} />
        <StatsGrid />
        <section className="mx-auto mt-16 grid w-full max-w-6xl gap-5 px-6 md:grid-cols-2 lg:grid-cols-4">
          {cards.map((card) => (
            <article
              key={card.title}
              className="rounded-2xl border border-border-color bg-card-background p-5 shadow-[0_1px_8px_rgba(16,19,35,0.05)]"
            >
              <h3 className="text-lg font-semibold">{card.title}</h3>
              <p className="mt-3 text-sm text-muted-foreground">{card.text}</p>
              <p className="mt-5 text-sm font-medium text-brand">{card.link}</p>
            </article>
          ))}
        </section>
      </main>
      <footer className="mt-auto border-t border-border-color py-6 text-center text-sm text-muted-foreground">
        GoRouter Dashboard - OpenRouter-style demo frontend
      </footer>
      <AuthDialog open={dialogOpen} onClose={() => setDialogOpen(false)} />
    </>
  );
}

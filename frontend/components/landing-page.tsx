"use client";

import { motion } from "framer-motion";
import {
  Activity,
  BarChart3,
  ChevronDown,
  Eye,
  Lock,
  Shield,
  X,
  Zap
} from "lucide-react";
import Link from "next/link";
import { FormEvent, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

type JSONRecord = Record<string, unknown>;

function parseResponse(rawBody: string): JSONRecord {
  if (!rawBody) {
    return {};
  }
  try {
    return JSON.parse(rawBody) as JSONRecord;
  } catch {
    return { raw: rawBody };
  }
}

async function postJSON(path: string, payload: unknown): Promise<JSONRecord> {
  const response = await fetch(path, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  const body = parseResponse(await response.text());

  if (!response.ok) {
    const message =
      (typeof body.error === "string" && body.error) ||
      (typeof body.message === "string" && body.message) ||
      `Request failed with HTTP ${response.status}`;
    throw new Error(message);
  }

  return body;
}

function ownerNameFromEmail(email: string): string {
  const localPart = email.split("@")[0]?.trim();
  if (!localPart) {
    return "Owner";
  }
  return localPart
    .split(/[._-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function SignupModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [activeForm, setActiveForm] = useState<"org" | "user">("org");
  const [orgName, setOrgName] = useState("");
  const [orgEmail, setOrgEmail] = useState("");
  const [userEmail, setUserEmail] = useState("");
  const [orgId, setOrgId] = useState("");

  const [orgBusy, setOrgBusy] = useState(false);
  const [userBusy, setUserBusy] = useState(false);

  const [orgMessage, setOrgMessage] = useState<string | null>(null);
  const [userMessage, setUserMessage] = useState<string | null>(null);
  const [orgError, setOrgError] = useState<string | null>(null);
  const [userError, setUserError] = useState<string | null>(null);

  const submitOrgSignup = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setOrgBusy(true);
    setOrgError(null);
    setOrgMessage(null);

    try {
      const createOrgResult = await postJSON("/v1/control/orgs", {
        org_name: orgName,
        owner_email: orgEmail,
        owner_name: ownerNameFromEmail(orgEmail)
      });

      const createdOrgId = String(createOrgResult.org_id ?? "");
      if (!createdOrgId) {
        throw new Error("Organization was created but org_id was not returned.");
      }

      setOrgId(createdOrgId);
      if (!userEmail) {
        setUserEmail(orgEmail);
      }
      setActiveForm("user");

      try {
        await postJSON("/v1/control/auth/magic-link/request", {
          org_id: createdOrgId,
          email: orgEmail
        });
        setOrgMessage(`Signup started. Check ${orgEmail} for the email link.`);
      } catch (error) {
        setOrgMessage("Organization created. Retry sending the sign-in link below.");
        setOrgError(error instanceof Error ? error.message : "Unable to send sign-in link.");
      }
    } catch (error) {
      setOrgError(error instanceof Error ? error.message : "Unable to start organization signup.");
    } finally {
      setOrgBusy(false);
    }
  };

  const submitUserSignup = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setUserBusy(true);
    setUserError(null);
    setUserMessage(null);

    try {
      const payload: { email: string; org_id?: string } = { email: userEmail };
      if (orgId.trim()) {
        payload.org_id = orgId.trim();
      }

      await postJSON("/v1/control/auth/magic-link/request", payload);

      setUserMessage(`Signup started. Check ${userEmail} for the email link.`);
    } catch (error) {
      setUserError(error instanceof Error ? error.message : "Unable to start user signup.");
    } finally {
      setUserBusy(false);
    }
  };

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <button
        type="button"
        aria-label="Close signup modal"
        className="absolute inset-0 bg-black/70 backdrop-blur-sm"
        onClick={onClose}
      />
      <motion.div
        initial={{ opacity: 0, y: 12, scale: 0.98 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        transition={{ duration: 0.2 }}
        className="relative z-10 w-full max-w-4xl overflow-hidden rounded-2xl border border-neutral-800 bg-neutral-950/95 shadow-2xl"
      >
        <div className="flex items-center justify-between border-b border-neutral-800 px-6 py-4">
          <div>
            <h3 className="text-xl font-semibold text-neutral-100">Open Console Access</h3>
            <p className="text-sm text-neutral-400">Sign up and check your email for the login link.</p>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="text-neutral-300 hover:bg-neutral-800 hover:text-neutral-100"
            onClick={onClose}
          >
            <X className="size-4" />
          </Button>
        </div>

        <div className="space-y-4 p-6">
          <div className="inline-flex w-full rounded-lg border border-neutral-800 bg-neutral-900/60 p-1">
            <button
              type="button"
              onClick={() => setActiveForm("org")}
              className={cn(
                "flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors",
                activeForm === "org"
                  ? "bg-neutral-100 text-neutral-900"
                  : "text-neutral-300 hover:bg-neutral-800 hover:text-neutral-100"
              )}
            >
              Signup Org
            </button>
            <button
              type="button"
              onClick={() => setActiveForm("user")}
              className={cn(
                "flex-1 rounded-md px-4 py-2 text-sm font-medium transition-colors",
                activeForm === "user"
                  ? "bg-neutral-100 text-neutral-900"
                  : "text-neutral-300 hover:bg-neutral-800 hover:text-neutral-100"
              )}
            >
              Signin User
            </button>
          </div>

          {activeForm === "org" ? (
            <form
              onSubmit={submitOrgSignup}
              className="space-y-4 rounded-xl border border-neutral-800 bg-neutral-900/50 p-5"
            >
              <h4 className="text-lg font-semibold text-neutral-100">Signup Org</h4>
              <p className="text-sm text-neutral-400">Create your organization with owner email.</p>
              <div className="grid gap-2">
                <Label htmlFor="signup-org-name" className="text-neutral-300">
                  Org Name
                </Label>
                <Input
                  id="signup-org-name"
                  value={orgName}
                  onChange={(event) => setOrgName(event.target.value)}
                  required
                  placeholder="Acme"
                  className="border-neutral-800 bg-neutral-950/60 text-neutral-100 placeholder:text-neutral-600"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="signup-org-email" className="text-neutral-300">
                  Org Email ID
                </Label>
                <Input
                  id="signup-org-email"
                  type="email"
                  value={orgEmail}
                  onChange={(event) => setOrgEmail(event.target.value)}
                  required
                  placeholder="owner@acme.com"
                  className="border-neutral-800 bg-neutral-950/60 text-neutral-100 placeholder:text-neutral-600"
                />
              </div>
              <Button
                type="submit"
                className="w-full bg-neutral-100 text-neutral-900 hover:bg-white"
                disabled={orgBusy}
              >
                {orgBusy ? "Starting..." : "Signup Org"}
              </Button>
              {orgId ? <p className="text-xs text-neutral-500">Organization ID: {orgId}</p> : null}
              {orgMessage ? <p className="text-sm text-emerald-400">{orgMessage}</p> : null}
              {orgError ? <p className="text-sm text-red-400">{orgError}</p> : null}
            </form>
          ) : (
            <form
              onSubmit={submitUserSignup}
              className="space-y-4 rounded-xl border border-neutral-800 bg-neutral-900/50 p-5"
            >
              <h4 className="text-lg font-semibold text-neutral-100">Signin User</h4>
              <p className="text-sm text-neutral-400">
                Enter user email to sign in. If this email belongs to multiple orgs, provide org ID.
              </p>
              <div className="grid gap-2">
                <Label htmlFor="signin-org-id" className="text-neutral-300">
                  Org ID (Optional)
                </Label>
                <Input
                  id="signin-org-id"
                  value={orgId}
                  onChange={(event) => setOrgId(event.target.value)}
                  placeholder="org_xxx"
                  className="border-neutral-800 bg-neutral-950/60 text-neutral-100 placeholder:text-neutral-600"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="signup-user-email" className="text-neutral-300">
                  Email ID
                </Label>
                <Input
                  id="signup-user-email"
                  type="email"
                  value={userEmail}
                  onChange={(event) => setUserEmail(event.target.value)}
                  required
                  placeholder="member@acme.com"
                  className="border-neutral-800 bg-neutral-950/60 text-neutral-100 placeholder:text-neutral-600"
                />
              </div>
              <Button
                type="submit"
                className="w-full bg-neutral-100 text-neutral-900 hover:bg-white"
                disabled={userBusy}
              >
                {userBusy ? "Starting..." : "Signin User"}
              </Button>
              {userMessage ? <p className="text-sm text-emerald-400">{userMessage}</p> : null}
              {userError ? <p className="text-sm text-red-400">{userError}</p> : null}
            </form>
          )}
        </div>

        <div className="flex flex-col gap-4 border-t border-neutral-800 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-sm text-neutral-400">
            For both org and user signup, check the email inbox (and spam folder) for the sign-in email.
          </p>
          <Button asChild variant="outline" className="border-neutral-700 text-neutral-200 hover:bg-neutral-800">
            <Link href="/base/console" onClick={onClose}>
              Continue to Console
            </Link>
          </Button>
        </div>
      </motion.div>
    </div>
  );
}

function FloatingPaths({ position }: { position: number }) {
  const paths = Array.from({ length: 36 }, (_, i) => ({
    id: i,
    d: `M-${380 - i * 5 * position} -${189 + i * 6}C-${
      380 - i * 5 * position
    } -${189 + i * 6} -${312 - i * 5 * position} ${216 - i * 6} ${152 - i * 5 * position} ${
      343 - i * 6
    }C${616 - i * 5 * position} ${470 - i * 6} ${684 - i * 5 * position} ${875 - i * 6} ${
      684 - i * 5 * position
    } ${875 - i * 6}`,
    width: 0.5 + i * 0.03
  }));

  return (
    <div className="pointer-events-none absolute inset-0">
      <svg className="h-full w-full text-neutral-800 dark:text-neutral-200" viewBox="0 0 696 316" fill="none">
        <title>Background Paths</title>
        {paths.map((path) => (
          <motion.path
            key={path.id}
            d={path.d}
            stroke="currentColor"
            strokeWidth={path.width}
            strokeOpacity={0.05 + path.id * 0.015}
            initial={{ pathLength: 0.3, opacity: 0.6 }}
            animate={{
              pathLength: 1,
              opacity: [0.2, 0.4, 0.2],
              pathOffset: [0, 1, 0]
            }}
            transition={{
              duration: 20 + Math.random() * 10,
              repeat: Number.POSITIVE_INFINITY,
              ease: "linear"
            }}
          />
        ))}
      </svg>
    </div>
  );
}

type FAQItem = {
  question: string;
  answer: string;
};

function FAQSection() {
  const [openIndex, setOpenIndex] = useState<number | null>(0);

  const faqs: FAQItem[] = [
    {
      question: "How does the observability gateway work?",
      answer:
        "Our gateway sits between your developers and AI models, capturing every interaction, token usage, and performance metric in real-time."
    },
    {
      question: "What AI models are supported?",
      answer:
        "We support major hosted and open-source models. The gateway is model-agnostic and integrates with API-based providers."
    },
    {
      question: "How do you ensure data security?",
      answer:
        "All traffic is encrypted in transit and data is encrypted at rest. Retention controls let you define what is kept and for how long."
    },
    {
      question: "Can I track costs per team or project?",
      answer:
        "Yes. Cost and usage can be attributed to org, team, project, and user dimensions for operational and budget reporting."
    },
    {
      question: "What kind of insights can I expect?",
      answer:
        "You get usage patterns, cost outliers, latency trends, model comparison views, and policy/compliance signals."
    }
  ];

  return (
    <section className="relative overflow-hidden bg-neutral-950 py-24">
      <div className="absolute inset-0 opacity-30">
        <FloatingPaths position={0.5} />
      </div>
      <div className="container relative z-10 mx-auto px-4 md:px-6">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          viewport={{ once: true }}
          className="mx-auto max-w-3xl"
        >
          <h2 className="mb-4 text-center text-4xl font-bold text-neutral-100 md:text-5xl">
            Frequently Asked Questions
          </h2>
          <p className="mb-12 text-center text-lg text-neutral-400">
            Everything you need to know about the observability platform
          </p>

          <div className="space-y-4">
            {faqs.map((faq, index) => (
              <motion.div
                key={faq.question}
                initial={{ opacity: 0, y: 10 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.4, delay: index * 0.1 }}
                viewport={{ once: true }}
                className="overflow-hidden rounded-lg border border-neutral-800 bg-neutral-900/50 backdrop-blur-sm"
              >
                <button
                  onClick={() => setOpenIndex(openIndex === index ? null : index)}
                  className="flex w-full items-center justify-between px-6 py-5 text-left transition-colors hover:bg-neutral-800/30"
                >
                  <span className="pr-4 text-lg font-semibold text-neutral-100">{faq.question}</span>
                  <ChevronDown
                    className={cn(
                      "h-5 w-5 shrink-0 text-neutral-400 transition-transform",
                      openIndex === index && "rotate-180"
                    )}
                  />
                </button>
                <motion.div
                  initial={false}
                  animate={{
                    height: openIndex === index ? "auto" : 0,
                    opacity: openIndex === index ? 1 : 0
                  }}
                  transition={{ duration: 0.3 }}
                  className="overflow-hidden"
                >
                  <div className="px-6 pb-5 leading-relaxed text-neutral-400">{faq.answer}</div>
                </motion.div>
              </motion.div>
            ))}
          </div>
        </motion.div>
      </div>
    </section>
  );
}

function HeroSection({ onOpenConsole }: { onOpenConsole: () => void }) {
  const words = "Observe How Your Developers Use AI".split(" ");

  return (
    <section className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-neutral-950">
      <div className="absolute inset-0">
        <FloatingPaths position={1} />
        <FloatingPaths position={-1} />
      </div>

      <div className="container relative z-10 mx-auto px-4 text-center md:px-6">
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 2 }}
          className="mx-auto max-w-5xl"
        >
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.2, duration: 0.6 }}
            className="mb-6 inline-block rounded-full border border-neutral-700 bg-neutral-900/50 px-4 py-2 backdrop-blur-sm"
          >
            <span className="text-sm font-medium text-neutral-300">AI Powered Observability</span>
          </motion.div>

          <h1 className="mb-8 text-5xl font-bold tracking-tighter sm:text-6xl md:text-7xl lg:text-8xl">
            {words.map((word, wordIndex) => (
              <span key={word} className="mr-4 inline-block last:mr-0">
                {word.split("").map((letter, letterIndex) => (
                  <motion.span
                    key={`${word}-${letterIndex}`}
                    initial={{ y: 100, opacity: 0 }}
                    animate={{ y: 0, opacity: 1 }}
                    transition={{
                      delay: wordIndex * 0.1 + letterIndex * 0.03,
                      type: "spring",
                      stiffness: 150,
                      damping: 25
                    }}
                    className="inline-block bg-gradient-to-r from-neutral-100 to-neutral-400 bg-clip-text text-transparent"
                  >
                    {letter}
                  </motion.span>
                ))}
              </span>
            ))}
          </h1>

          <motion.p
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.8, duration: 0.6 }}
            className="mx-auto mb-12 max-w-3xl text-xl leading-relaxed text-neutral-400 md:text-2xl"
          >
            An observability gateway that shows how your developers use AI in development, with
            real-time usage, cost, and performance visibility.
          </motion.p>

          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 1, duration: 0.6 }}
            className="group relative inline-block overflow-hidden rounded-2xl bg-gradient-to-b from-neutral-800/50 to-neutral-900/50 p-px shadow-lg backdrop-blur-lg transition-shadow duration-300 hover:shadow-xl"
          >
            <Button
              type="button"
              variant="ghost"
              className="rounded-[1.15rem] border border-neutral-700 bg-neutral-900/95 px-8 py-6 text-lg font-semibold text-neutral-100 backdrop-blur-md transition-all duration-300 hover:bg-neutral-800 group-hover:-translate-y-0.5 hover:shadow-md"
              onClick={onOpenConsole}
            >
              <span className="opacity-90 transition-opacity group-hover:opacity-100">Open Console</span>
              <span className="ml-3 opacity-70 transition-all duration-300 group-hover:translate-x-1.5 group-hover:opacity-100">
                →
              </span>
            </Button>
          </motion.div>
        </motion.div>
      </div>
    </section>
  );
}

function FeaturesSection() {
  const features = [
    {
      title: "Real-time Observability",
      description:
        "Monitor every AI interaction as it happens. Track token usage, latency, and model performance in real-time.",
      icon: <Eye className="h-6 w-6" />
    },
    {
      title: "Cost Analytics",
      description:
        "Understand AI spending with granular cost breakdowns by team, project, and developer.",
      icon: <BarChart3 className="h-6 w-6" />
    },
    {
      title: "Security & Compliance",
      description:
        "Ensure AI usage meets security standards with built-in compliance monitoring and alerts.",
      icon: <Shield className="h-6 w-6" />
    },
    {
      title: "Performance Insights",
      description:
        "Identify bottlenecks and optimize workflows with detailed latency and quality metrics.",
      icon: <Zap className="h-6 w-6" />
    },
    {
      title: "Access Control",
      description:
        "Manage model access with fine-grained permissions, scoped keys, and audit-friendly history.",
      icon: <Lock className="h-6 w-6" />
    },
    {
      title: "Usage Patterns",
      description:
        "Discover usage trends across teams and use productivity recommendations to improve outcomes.",
      icon: <Activity className="h-6 w-6" />
    }
  ];

  return (
    <section className="relative overflow-hidden bg-neutral-900 py-24">
      <div className="absolute inset-0 opacity-20">
        <FloatingPaths position={-0.5} />
      </div>
      <div className="container relative z-10 mx-auto px-4 md:px-6">
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6 }}
          viewport={{ once: true }}
          className="mb-16 text-center"
        >
          <h2 className="mb-4 text-4xl font-bold text-neutral-100 md:text-5xl">Complete Visibility</h2>
          <p className="mx-auto max-w-2xl text-lg text-neutral-400">
            Everything needed to understand and optimize your team&apos;s AI development workflow.
          </p>
        </motion.div>

        <div className="mx-auto grid max-w-7xl grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
          {features.map((feature, index) => (
            <motion.div
              key={feature.title}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: index * 0.1 }}
              viewport={{ once: true }}
              className="group relative"
            >
              <div className="h-full rounded-xl border border-neutral-800 bg-neutral-950/50 p-8 backdrop-blur-sm transition-all duration-300 hover:border-neutral-700 hover:bg-neutral-900/50">
                <div className="mb-4 inline-flex rounded-lg bg-neutral-800/50 p-3 text-neutral-300 transition-all duration-300 group-hover:bg-neutral-700/50 group-hover:text-neutral-100">
                  {feature.icon}
                </div>
                <h3 className="mb-3 text-xl font-semibold text-neutral-100 transition-colors group-hover:text-white">
                  {feature.title}
                </h3>
                <p className="leading-relaxed text-neutral-400">{feature.description}</p>
              </div>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}

function ContactSection() {
  return (
    <section className="relative overflow-hidden bg-neutral-950 py-24">
      <div className="absolute inset-0 opacity-20">
        <FloatingPaths position={1} />
      </div>
      <div className="container relative z-10 mx-auto px-4 md:px-6">
        <div className="mx-auto flex max-w-screen-xl flex-col justify-between gap-10 lg:flex-row lg:gap-20">
          <motion.div
            initial={{ opacity: 0, x: -20 }}
            whileInView={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.6 }}
            viewport={{ once: true }}
            className="mx-auto flex max-w-sm flex-col justify-between gap-10"
          >
            <div className="text-center lg:text-left">
              <h2 className="mb-4 text-4xl font-bold text-neutral-100 md:text-5xl">Get in Touch</h2>
              <p className="text-lg leading-relaxed text-neutral-400">
                Ready to gain visibility into AI development? Let&apos;s discuss how this platform can
                help your team.
              </p>
            </div>
            <div className="mx-auto w-fit lg:mx-0">
              <h3 className="mb-6 text-center text-2xl font-semibold text-neutral-200 lg:text-left">
                Contact Details
              </h3>
              <ul className="ml-4 list-disc space-y-2 text-neutral-400">
                <li>
                  <span className="font-bold text-neutral-300">Email: </span>
                  <a
                    href="mailto:hello@aigateway.dev"
                    className="underline transition-colors hover:text-neutral-200"
                  >
                    hello@aigateway.dev
                  </a>
                </li>
                <li>
                  <span className="font-bold text-neutral-300">Web: </span>
                  <a
                    href="https://aigateway.dev"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline transition-colors hover:text-neutral-200"
                  >
                    aigateway.dev
                  </a>
                </li>
              </ul>
            </div>
          </motion.div>

          <motion.form
            initial={{ opacity: 0, x: 20 }}
            whileInView={{ opacity: 1, x: 0 }}
            transition={{ duration: 0.6 }}
            viewport={{ once: true }}
            className="mx-auto flex max-w-screen-md flex-col gap-6 rounded-xl border border-neutral-800 bg-neutral-900/50 p-8 backdrop-blur-sm md:p-10"
          >
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid w-full items-center gap-2">
                <Label htmlFor="firstname" className="text-neutral-300">
                  First Name
                </Label>
                <Input
                  type="text"
                  id="firstname"
                  placeholder="John"
                  className="border-neutral-800 bg-neutral-950/50 text-neutral-100 placeholder:text-neutral-600"
                />
              </div>
              <div className="grid w-full items-center gap-2">
                <Label htmlFor="lastname" className="text-neutral-300">
                  Last Name
                </Label>
                <Input
                  type="text"
                  id="lastname"
                  placeholder="Doe"
                  className="border-neutral-800 bg-neutral-950/50 text-neutral-100 placeholder:text-neutral-600"
                />
              </div>
            </div>
            <div className="grid w-full items-center gap-2">
              <Label htmlFor="email" className="text-neutral-300">
                Email
              </Label>
              <Input
                type="email"
                id="email"
                placeholder="john@company.com"
                className="border-neutral-800 bg-neutral-950/50 text-neutral-100 placeholder:text-neutral-600"
              />
            </div>
            <div className="grid w-full items-center gap-2">
              <Label htmlFor="company" className="text-neutral-300">
                Company
              </Label>
              <Input
                type="text"
                id="company"
                placeholder="Your Company"
                className="border-neutral-800 bg-neutral-950/50 text-neutral-100 placeholder:text-neutral-600"
              />
            </div>
            <div className="grid w-full gap-2">
              <Label htmlFor="message" className="text-neutral-300">
                Message
              </Label>
              <Textarea
                id="message"
                placeholder="Tell us about your AI observability needs..."
                className="min-h-[120px] border-neutral-800 bg-neutral-950/50 text-neutral-100 placeholder:text-neutral-600"
              />
            </div>
            <Button className="w-full bg-neutral-100 font-semibold text-neutral-900 hover:bg-white">
              Send Message
            </Button>
          </motion.form>
        </div>
      </div>
    </section>
  );
}

function Footer() {
  return (
    <footer className="border-t border-neutral-800 bg-neutral-950 py-12">
      <div className="container mx-auto px-4 md:px-6">
        <div className="mb-8 grid grid-cols-1 gap-8 md:grid-cols-4">
          <div className="col-span-1 md:col-span-2">
            <h3 className="mb-4 text-2xl font-bold text-neutral-100">AI Gateway</h3>
            <p className="max-w-md leading-relaxed text-neutral-400">
              Observability-driven gateway helping engineering teams understand and optimize AI
              development workflows.
            </p>
          </div>
          <div>
            <h4 className="mb-4 font-semibold text-neutral-200">Product</h4>
            <ul className="space-y-2">
              <li>
                <a href="#features" className="text-neutral-400 transition-colors hover:text-neutral-200">
                  Features
                </a>
              </li>
              <li>
                <a href="#faq" className="text-neutral-400 transition-colors hover:text-neutral-200">
                  FAQ
                </a>
              </li>
              <li>
                <Link href="/base/console" className="text-neutral-400 transition-colors hover:text-neutral-200">
                  Console
                </Link>
              </li>
            </ul>
          </div>
          <div>
            <h4 className="mb-4 font-semibold text-neutral-200">Company</h4>
            <ul className="space-y-2">
              <li>
                <span className="text-neutral-400">
                  About
                </span>
              </li>
              <li>
                <span className="text-neutral-400">
                  Blog
                </span>
              </li>
              <li>
                <span className="text-neutral-400">
                  Contact
                </span>
              </li>
            </ul>
          </div>
        </div>
        <div className="border-t border-neutral-800 pt-8 text-center text-sm text-neutral-500">
          <p>&copy; {new Date().getFullYear()} AI Gateway. All rights reserved.</p>
        </div>
      </div>
    </footer>
  );
}

export function LandingPage() {
  const [isSignupOpen, setIsSignupOpen] = useState(false);

  return (
    <main className="bg-neutral-950">
      <HeroSection onOpenConsole={() => setIsSignupOpen(true)} />
      <div id="features">
        <FeaturesSection />
      </div>
      <ContactSection />
      <div id="faq">
        <FAQSection />
      </div>
      <Footer />
      <SignupModal open={isSignupOpen} onClose={() => setIsSignupOpen(false)} />
    </main>
  );
}

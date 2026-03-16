"use client";

import { AnimatePresence, motion } from "framer-motion";
import {
  Activity,
  AlertCircle,
  BarChart3,
  Bell,
  CheckCircle,
  ChevronLeft,
  Download,
  FileText,
  Home,
  Key,
  Layers,
  Menu,
  Plus,
  Search,
  Settings,
  Shield,
  TrendingDown,
  TrendingUp,
  User,
  Users,
  X,
  XCircle,
  Zap
} from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";

type Role = "org_owner" | "team_admin" | "member";

type Team = {
  id: string;
  name: string;
  memberCount: number;
  adminCount: number;
  policies: string[];
  models: string[];
};

type TeamMember = {
  userId: string;
  email: string;
  name: string;
  orgRole: string;
  teamRole: string;
  hasScope: boolean;
  createdAt: Date;
};

type TeamAdmin = {
  userId: string;
  email: string;
  name: string;
  orgRole: string;
  createdAt: Date;
};

type ApiKey = {
  id: string;
  name: string;
  key: string;
  teamId: string;
  userId: string;
  createdAt: Date;
  lastUsed?: Date;
  revoked: boolean;
};

type UsageData = {
  requests: number;
  inputTokens: number;
  outputTokens: number;
  errorRate: number;
  p50Latency: number;
  p95Latency: number;
  estimatedCost: number;
};

type EventItem = {
  id: string;
  type: string;
  user: string;
  description: string;
  timestamp: Date;
  status: "success" | "warning" | "error";
};

type SessionResponse = {
  org_id: string;
  user_id: string;
  role: Role;
};

type SummaryResponse = {
  teams: number;
  members: number;
  active_keys: number;
  policy_entries: number;
};

type ListResponse<T> = {
  items?: T[];
};

type TeamListItem = {
  id: string;
  name: string;
};

type PolicyRow = {
  provider: string;
  model: string;
  is_allowed: boolean;
};

type ProviderKey = {
  id: string;
  provider: string;
  keyKekID: string;
  isActive: boolean;
  createdAt: Date;
};

type UsageSummaryResponse = {
  request_count: number;
  request_tokens: number;
  response_tokens: number;
  error_rate: number;
  latency_p50_ms: number;
  latency_p95_ms: number;
};

const emptyUsageData: UsageData = {
  requests: 0,
  inputTokens: 0,
  outputTokens: 0,
  errorRate: 0,
  p50Latency: 0,
  p95Latency: 0,
  estimatedCost: 0
};

const routerSupportedProviders = ["openai", "claude", "anthropic", "gemini", "codex"] as const;

const cn = (...classes: Array<string | undefined | null | false>) => classes.filter(Boolean).join(" ");

function parseJSON(raw: string): unknown {
  if (!raw) {
    return {};
  }
  try {
    return JSON.parse(raw);
  } catch {
    return { raw };
  }
}

function responseError(body: unknown, status: number): string {
  if (typeof body === "object" && body) {
    const candidate = body as { error?: unknown; message?: unknown };
    if (typeof candidate.error === "string" && candidate.error) {
      return candidate.error;
    }
    if (typeof candidate.message === "string" && candidate.message) {
      return candidate.message;
    }
  }
  return `Request failed with HTTP ${status}`;
}

async function getJSON<T>(path: string): Promise<T> {
  return requestJSON<T>("GET", path);
}

async function requestJSON<T>(method: "GET" | "POST" | "PUT", path: string, payload?: unknown): Promise<T> {
  const response = await fetch(path, {
    method,
    credentials: "include",
    cache: "no-store",
    headers: payload ? { "Content-Type": "application/json" } : undefined,
    body: payload ? JSON.stringify(payload) : undefined
  });
  const body = parseJSON(await response.text());
  if (!response.ok) {
    throw new Error(responseError(body, response.status));
  }
  return body as T;
}

async function postJSON<T>(path: string, payload: unknown): Promise<T> {
  return requestJSON<T>("POST", path, payload);
}

async function putJSON<T>(path: string, payload: unknown): Promise<T> {
  return requestJSON<T>("PUT", path, payload);
}

function toArray<T>(value: T[] | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

function fromForRange(range: string, now: Date): Date {
  const msByRange: Record<string, number> = {
    "24h": 24 * 60 * 60 * 1000,
    "7d": 7 * 24 * 60 * 60 * 1000,
    "30d": 30 * 24 * 60 * 60 * 1000,
    "90d": 90 * 24 * 60 * 60 * 1000
  };
  const delta = msByRange[range] ?? msByRange["7d"];
  return new Date(now.getTime() - delta);
}

function formatRelativeTime(value: Date): string {
  const nowMs = Date.now();
  const deltaMs = Math.max(0, nowMs - value.getTime());
  const minutes = Math.floor(deltaMs / 60000);
  if (minutes < 1) {
    return "just now";
  }
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h ago`;
  }
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function describeEvent(type: string, teamID: string | null, userID: string | null, resourceID: string): string {
  switch (type) {
    case "org_created":
      return `Organization created (${resourceID}).`;
    case "team_created":
      return `Team created (${teamID ?? resourceID}).`;
    case "team_member_added":
      return `Added member ${userID ?? "unknown"} to team ${teamID ?? "unknown"}.`;
    case "team_admin_scoped":
      return `Granted team admin scope for ${userID ?? "unknown"} on ${teamID ?? "unknown"}.`;
    case "api_key_created":
      return `Created API key ${resourceID}.`;
    case "api_key_revoked":
      return `Revoked API key ${resourceID}.`;
    case "provider_key_created":
      return `Stored provider key ${resourceID}.`;
    case "org_policy_upserted":
      return `Updated org model policy ${resourceID}.`;
    case "team_policy_upserted":
      return `Updated team model policy ${resourceID}.`;
    default:
      return `${type} (${resourceID})`;
  }
}

function eventStatus(type: string): "success" | "warning" | "error" {
  if (type === "api_key_revoked") {
    return "warning";
  }
  return "success";
}

function Card({
  children,
  className,
  onClick
}: {
  children: React.ReactNode;
  className?: string;
  onClick?: () => void;
}) {
  const classes = cn(
    "rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] p-6",
    onClick ? "block w-full text-left" : undefined,
    className
  );

  if (onClick) {
    return (
      <button type="button" className={classes} onClick={onClick}>
        {children}
      </button>
    );
  }

  return <div className={classes}>{children}</div>;
}

function KPICard({
  title,
  value,
  icon,
  trend,
  subtitle
}: {
  title: string;
  value: string | number;
  icon: React.ReactNode;
  trend?: number;
  subtitle?: string;
}) {
  return (
    <Card className="transition-colors hover:border-[#3a3a3a]">
      <div className="flex items-start justify-between">
        <div className="flex-1">
          <p className="mb-1 text-sm text-[#888]">{title}</p>
          <p className="mb-1 text-2xl font-semibold text-white">{value}</p>
          {subtitle ? <p className="text-xs text-[#666]">{subtitle}</p> : null}
        </div>
        <div className="rounded-lg bg-[#2a2a2a] p-3">{icon}</div>
      </div>
      {trend !== undefined ? (
        <div className="mt-3 flex items-center text-xs">
          {trend > 0 ? (
            <>
              <TrendingUp className="mr-1 h-3 w-3 text-green-500" />
              <span className="text-green-500">+{trend}%</span>
            </>
          ) : (
            <>
              <TrendingDown className="mr-1 h-3 w-3 text-red-500" />
              <span className="text-red-500">{trend}%</span>
            </>
          )}
          <span className="ml-1 text-[#666]">vs last period</span>
        </div>
      ) : null}
    </Card>
  );
}

function Button({
  children,
  onClick,
  variant = "primary",
  size = "md",
  className,
  disabled
}: {
  children: React.ReactNode;
  onClick?: () => void;
  variant?: "primary" | "secondary" | "ghost";
  size?: "sm" | "md" | "lg";
  className?: string;
  disabled?: boolean;
}) {
  const baseStyles =
    "inline-flex items-center justify-center gap-2 rounded-lg font-medium transition-all";
  const variants = {
    primary: "bg-white text-black hover:bg-[#e0e0e0]",
    secondary: "bg-[#2a2a2a] text-white hover:bg-[#3a3a3a]",
    ghost: "bg-transparent text-[#888] hover:bg-[#2a2a2a] hover:text-white"
  };
  const sizes = {
    sm: "px-3 py-1.5 text-sm",
    md: "px-4 py-2 text-sm",
    lg: "px-6 py-3 text-base"
  };

  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className={cn(
        baseStyles,
        variants[variant],
        sizes[size],
        disabled && "cursor-not-allowed opacity-50",
        className
      )}
      type="button"
    >
      {children}
    </button>
  );
}

function Badge({
  children,
  variant = "default"
}: {
  children: React.ReactNode;
  variant?: "default" | "success" | "warning" | "error";
}) {
  const variants = {
    default: "bg-[#2a2a2a] text-[#888]",
    success: "bg-green-500/10 text-green-500",
    warning: "bg-yellow-500/10 text-yellow-500",
    error: "bg-red-500/10 text-red-500"
  };

  return <span className={cn("rounded px-2 py-1 text-xs font-medium", variants[variant])}>{children}</span>;
}

function EmptyState({
  icon,
  title,
  description,
  action
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <div className="mb-4 rounded-full bg-[#2a2a2a] p-4">{icon}</div>
      <h3 className="mb-2 text-lg font-semibold text-white">{title}</h3>
      <p className="mb-4 max-w-md text-sm text-[#888]">{description}</p>
      {action}
    </div>
  );
}

function Navigation({
  activeTab,
  setActiveTab,
  role,
  isMobileMenuOpen,
  setIsMobileMenuOpen
}: {
  activeTab: string;
  setActiveTab: (tab: string) => void;
  role: Role;
  isMobileMenuOpen: boolean;
  setIsMobileMenuOpen: (open: boolean) => void;
}) {
  const navItems: Array<{
    id: string;
    label: string;
    icon: React.ComponentType<{ className?: string }>;
    roles: Role[];
  }> = [
    { id: "overview", label: "Overview", icon: Home, roles: ["org_owner", "team_admin", "member"] },
    { id: "teams", label: "Teams", icon: Users, roles: ["org_owner", "team_admin"] },
    { id: "api-keys", label: "API Keys", icon: Key, roles: ["org_owner"] },
    { id: "models", label: "Models & Policies", icon: Shield, roles: ["org_owner", "team_admin"] },
    { id: "usage", label: "Usage", icon: Activity, roles: ["org_owner"] },
    { id: "events", label: "Events", icon: FileText, roles: ["org_owner"] }
  ];

  const visibleItems = navItems.filter((item) => item.roles.includes(role));

  return (
    <>
      <nav className="hidden items-center gap-1 md:flex">
        {visibleItems.map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.id}
              onClick={() => setActiveTab(item.id)}
              className={cn(
                "flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-all",
                activeTab === item.id
                  ? "bg-[#2a2a2a] text-white"
                  : "text-[#888] hover:bg-[#1a1a1a] hover:text-white"
              )}
              type="button"
            >
              <Icon className="h-4 w-4" />
              {item.label}
            </button>
          );
        })}
      </nav>

      <button
        onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
        className="p-2 text-[#888] hover:text-white md:hidden"
        type="button"
      >
        {isMobileMenuOpen ? <X className="h-6 w-6" /> : <Menu className="h-6 w-6" />}
      </button>

      <AnimatePresence>
        {isMobileMenuOpen ? (
          <motion.div
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -20 }}
            className="absolute left-0 right-0 top-16 z-50 border-b border-[#2a2a2a] bg-[#0a0a0a] p-4 md:hidden"
          >
            {visibleItems.map((item) => {
              const Icon = item.icon;
              return (
                <button
                  key={item.id}
                  onClick={() => {
                    setActiveTab(item.id);
                    setIsMobileMenuOpen(false);
                  }}
                  className={cn(
                    "mb-2 flex w-full items-center gap-2 rounded-lg px-4 py-3 text-sm font-medium transition-all",
                    activeTab === item.id
                      ? "bg-[#2a2a2a] text-white"
                      : "text-[#888] hover:bg-[#1a1a1a] hover:text-white"
                  )}
                  type="button"
                >
                  <Icon className="h-4 w-4" />
                  {item.label}
                </button>
              );
            })}
          </motion.div>
        ) : null}
      </AnimatePresence>
    </>
  );
}

function OverviewTab({
  role,
  loading,
  error,
  summary,
  teams,
  events
}: {
  role: Role;
  loading: boolean;
  error: string | null;
  summary: SummaryResponse | null;
  teams: Team[];
  events: EventItem[];
}) {
  if (role === "member") {
    return (
      <div className="space-y-6">
        <Card>
          <h2 className="mb-4 text-xl font-semibold text-white">My Access</h2>
          <EmptyState
            icon={<Key className="h-6 w-6 text-[#888]" />}
            title="No API Keys Available"
            description="You do not have permission to create API keys. Contact your team admin or org owner."
          />
        </Card>
      </div>
    );
  }

  if (loading && !summary) {
    return <p className="text-sm text-[#888]">Loading overview...</p>;
  }

  if (error && !summary) {
    return <p className="text-sm text-red-400">{error}</p>;
  }

  const totalMembers = summary?.members ?? teams.reduce((acc, team) => acc + team.memberCount, 0);

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4">
        <KPICard
          title="Teams"
          value={summary?.teams ?? teams.length}
          icon={<Users className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Total Members"
          value={totalMembers}
          icon={<User className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Active Keys"
          value={summary?.active_keys ?? 0}
          icon={<Key className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Policy Entries"
          value={summary?.policy_entries ?? 0}
          icon={<Shield className="h-5 w-5 text-white" />}
        />
      </div>

      <Card>
        <h2 className="mb-4 text-lg font-semibold text-white">Last Activity</h2>
        {events.length === 0 ? (
          <p className="text-sm text-[#888]">No events yet.</p>
        ) : (
          <div className="space-y-3">
            {events.slice(0, 5).map((entry) => (
              <div
                key={entry.id}
                className="flex items-center justify-between rounded-lg bg-[#0a0a0a] p-3"
              >
                <div className="flex items-center gap-3">
                  <div
                    className={cn(
                      "h-2 w-2 rounded-full",
                      entry.status === "warning"
                        ? "bg-yellow-500"
                        : entry.status === "error"
                          ? "bg-red-500"
                          : "bg-green-500"
                    )}
                  />
                  <div>
                    <p className="text-sm text-white">{entry.description}</p>
                    <p className="text-xs text-[#666]">{entry.user}</p>
                  </div>
                </div>
                <span className="text-xs text-[#666]">{formatRelativeTime(entry.timestamp)}</span>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}

function TeamsTab({
  role,
  orgID,
  teams,
  loading,
  error,
  creating,
  onCreateTeam,
  onRefresh
}: {
  role: Role;
  orgID: string;
  teams: Team[];
  loading: boolean;
  error: string | null;
  creating: boolean;
  onCreateTeam: (name: string) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const canManageMembers = role === "org_owner" || role === "team_admin";
  const [selectedTeam, setSelectedTeam] = useState<Team | null>(null);
  const [newTeamName, setNewTeamName] = useState("");
  const [createError, setCreateError] = useState<string | null>(null);
  const [teamMembers, setTeamMembers] = useState<TeamMember[]>([]);
  const [teamAdmins, setTeamAdmins] = useState<TeamAdmin[]>([]);
  const [teamPolicies, setTeamPolicies] = useState<PolicyRow[]>([]);
  const [teamModels, setTeamModels] = useState<string[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);
  const [memberName, setMemberName] = useState("");
  const [memberEmail, setMemberEmail] = useState("");
  const [memberUserID, setMemberUserID] = useState("");
  const [addingMember, setAddingMember] = useState(false);
  const [promoteUserID, setPromoteUserID] = useState("");
  const [promotingAdminID, setPromotingAdminID] = useState<string | null>(null);
  const [teamActionError, setTeamActionError] = useState<string | null>(null);
  const [teamActionMessage, setTeamActionMessage] = useState<string | null>(null);

  const submitCreateTeam = async () => {
    const trimmed = newTeamName.trim();
    if (!trimmed) {
      setCreateError("Team name is required.");
      return;
    }
    setCreateError(null);
    try {
      await onCreateTeam(trimmed);
      setNewTeamName("");
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create team.");
    }
  };

  const loadTeamAccess = useCallback(
    async (teamID: string) => {
      if (!orgID) {
        setDetailError("Missing org session context.");
        return;
      }
      setDetailLoading(true);
      setDetailError(null);
      try {
        const [membersRes, adminsRes, policiesRes, modelsRes] = await Promise.all([
          getJSON<
            ListResponse<{
              user_id: string;
              email: string;
              name: string;
              org_role: string;
              team_role: string;
              has_scope: boolean;
              created_at: string;
            }>
          >(
            `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(teamID)}/members?limit=200`
          ),
          getJSON<
            ListResponse<{
              user_id: string;
              email: string;
              name: string;
              org_role: string;
              created_at: string;
            }>
          >(
            `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(teamID)}/admins?limit=200`
          ),
          getJSON<ListResponse<PolicyRow>>(
            `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(teamID)}/policies/models`
          ),
          getJSON<ListResponse<{ provider: string; model: string }>>(
            `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(teamID)}/policies/effective-models?limit=200`
          )
        ]);

        setTeamMembers(
          toArray(membersRes.items).map((entry) => ({
            userId: entry.user_id,
            email: entry.email,
            name: entry.name,
            orgRole: entry.org_role,
            teamRole: entry.team_role,
            hasScope: entry.has_scope,
            createdAt: new Date(entry.created_at)
          }))
        );
        setTeamAdmins(
          toArray(adminsRes.items).map((entry) => ({
            userId: entry.user_id,
            email: entry.email,
            name: entry.name,
            orgRole: entry.org_role,
            createdAt: new Date(entry.created_at)
          }))
        );
        setTeamPolicies(toArray(policiesRes.items));
        setTeamModels(toArray(modelsRes.items).map((entry) => `${entry.provider}/${entry.model}`));
      } catch (err) {
        setTeamMembers([]);
        setTeamAdmins([]);
        setTeamPolicies([]);
        setTeamModels([]);
        setDetailError(err instanceof Error ? err.message : "Failed to load team members.");
      } finally {
        setDetailLoading(false);
      }
    },
    [orgID]
  );

  useEffect(() => {
    if (!selectedTeam) {
      setTeamMembers([]);
      setTeamAdmins([]);
      setTeamPolicies([]);
      setTeamModels([]);
      setDetailError(null);
      setTeamActionError(null);
      setTeamActionMessage(null);
      return;
    }
    void loadTeamAccess(selectedTeam.id);
  }, [loadTeamAccess, selectedTeam]);

  const submitAddMember = async () => {
    if (!selectedTeam) {
      return;
    }
    if (!memberEmail.trim() && !memberUserID.trim()) {
      setTeamActionError("Provide at least email or user_id.");
      return;
    }
    if (!orgID) {
      setTeamActionError("Missing org session context.");
      return;
    }

    setAddingMember(true);
    setTeamActionError(null);
    setTeamActionMessage(null);
    try {
      const payload: Record<string, string> = {
        role: "member"
      };
      if (memberName.trim()) {
        payload.name = memberName.trim();
      }
      if (memberEmail.trim()) {
        payload.email = memberEmail.trim().toLowerCase();
      }
      if (memberUserID.trim()) {
        payload.user_id = memberUserID.trim();
      }

      await postJSON(
        `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(selectedTeam.id)}/members`,
        payload
      );

      setMemberName("");
      setMemberEmail("");
      setMemberUserID("");
      setTeamActionMessage("Member added to team.");
      await Promise.all([loadTeamAccess(selectedTeam.id), onRefresh()]);
    } catch (err) {
      setTeamActionError(err instanceof Error ? err.message : "Failed to add team member.");
    } finally {
      setAddingMember(false);
    }
  };

  const submitPromoteToAdmin = async (userID: string) => {
    if (!selectedTeam) {
      return;
    }
    const trimmedUserID = userID.trim();
    if (!trimmedUserID) {
      setTeamActionError("user_id is required to assign team admin.");
      return;
    }
    if (!orgID) {
      setTeamActionError("Missing org session context.");
      return;
    }

    setPromotingAdminID(trimmedUserID);
    setTeamActionError(null);
    setTeamActionMessage(null);
    try {
      await postJSON(
        `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(selectedTeam.id)}/admins/${encodeURIComponent(trimmedUserID)}`,
        {}
      );
      setPromoteUserID("");
      setTeamActionMessage("Team admin scope granted.");
      await Promise.all([loadTeamAccess(selectedTeam.id), onRefresh()]);
    } catch (err) {
      setTeamActionError(err instanceof Error ? err.message : "Failed to assign team admin.");
    } finally {
      setPromotingAdminID(null);
    }
  };

  if (selectedTeam) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <button
            onClick={() => setSelectedTeam(null)}
            className="flex items-center gap-2 text-[#888] hover:text-white"
            type="button"
          >
            <ChevronLeft className="h-4 w-4" />
            Back to Teams
          </button>
        </div>

        <Card>
          <h2 className="mb-6 text-xl font-semibold text-white">{selectedTeam.name}</h2>
          <div className="mb-6 grid grid-cols-1 gap-4 md:grid-cols-3">
            <div className="rounded-lg bg-[#0a0a0a] p-4">
              <p className="mb-1 text-sm text-[#888]">Members</p>
              <p className="text-2xl font-semibold text-white">{teamMembers.length || selectedTeam.memberCount}</p>
            </div>
            <div className="rounded-lg bg-[#0a0a0a] p-4">
              <p className="mb-1 text-sm text-[#888]">Admins</p>
              <p className="text-2xl font-semibold text-white">{teamAdmins.length || selectedTeam.adminCount}</p>
            </div>
            <div className="rounded-lg bg-[#0a0a0a] p-4">
              <p className="mb-1 text-sm text-[#888]">Policies</p>
              <p className="text-2xl font-semibold text-white">{teamPolicies.length || selectedTeam.policies.length}</p>
            </div>
          </div>

          {canManageMembers ? (
            <div className="mb-6 rounded-lg border border-[#2a2a2a] bg-[#111] p-4">
              <h3 className="mb-3 font-medium text-white">Manage Team Access</h3>
              <div className="grid grid-cols-1 gap-2 md:grid-cols-3">
                <input
                  type="text"
                  value={memberName}
                  onChange={(event) => setMemberName(event.target.value)}
                  placeholder="username"
                  className="rounded-lg border border-[#2a2a2a] bg-[#0a0a0a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
                />
                <input
                  type="email"
                  value={memberEmail}
                  onChange={(event) => setMemberEmail(event.target.value)}
                  placeholder="user email"
                  className="rounded-lg border border-[#2a2a2a] bg-[#0a0a0a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
                />
                <input
                  type="text"
                  value={memberUserID}
                  onChange={(event) => setMemberUserID(event.target.value)}
                  placeholder="existing user_id (optional)"
                  className="rounded-lg border border-[#2a2a2a] bg-[#0a0a0a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
                />
              </div>
              <div className="mt-3 flex flex-wrap items-center gap-2">
                <Button variant="primary" size="sm" onClick={() => void submitAddMember()} disabled={addingMember}>
                  {addingMember ? "Adding..." : "Add Member"}
                </Button>
                <span className="text-xs text-[#666]">
                  Team admins can add members. Org owners can also promote team admins.
                </span>
              </div>

              {role === "org_owner" ? (
                <div className="mt-4 border-t border-[#2a2a2a] pt-4">
                  <h4 className="mb-2 text-sm font-medium text-white">Assign Team Admin</h4>
                  <div className="flex flex-wrap items-center gap-2">
                    <input
                      type="text"
                      value={promoteUserID}
                      onChange={(event) => setPromoteUserID(event.target.value)}
                      placeholder="user_id to promote"
                      className="rounded-lg border border-[#2a2a2a] bg-[#0a0a0a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
                    />
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => void submitPromoteToAdmin(promoteUserID)}
                      disabled={Boolean(promotingAdminID)}
                    >
                      {promotingAdminID ? "Assigning..." : "Make Team Admin"}
                    </Button>
                  </div>
                </div>
              ) : null}

              {teamActionMessage ? <p className="mt-3 text-sm text-green-400">{teamActionMessage}</p> : null}
              {teamActionError ? <p className="mt-3 text-sm text-red-400">{teamActionError}</p> : null}
            </div>
          ) : null}

          <div>
            <h3 className="mb-3 font-medium text-white">Effective Models</h3>
            <div className="flex flex-wrap gap-2">
              {teamModels.length > 0 ? (
                teamModels.map((model) => <Badge key={model}>{model}</Badge>)
              ) : selectedTeam.models.length > 0 ? (
                selectedTeam.models.map((model) => <Badge key={model}>{model}</Badge>)
              ) : (
                <span className="text-sm text-[#666]">No effective models.</span>
              )}
            </div>
          </div>

          <div className="mt-6 grid grid-cols-1 gap-4 lg:grid-cols-2">
            <div className="rounded-lg bg-[#0a0a0a] p-4">
              <h3 className="mb-3 font-medium text-white">Team Members</h3>
              {detailLoading ? <p className="text-sm text-[#888]">Loading members...</p> : null}
              {detailError ? <p className="text-sm text-red-400">{detailError}</p> : null}
              {!detailLoading && !detailError ? (
                <div className="space-y-2">
                  {teamMembers.length > 0 ? (
                    teamMembers.map((member) => (
                      <div key={member.userId} className="rounded-lg border border-[#1f1f1f] px-3 py-2">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <div>
                            <p className="text-sm text-white">{member.name || member.email}</p>
                            <p className="text-xs text-[#888]">{member.email}</p>
                            <p className="font-mono text-[11px] text-[#666]">{member.userId}</p>
                          </div>
                          <div className="flex items-center gap-2">
                            <Badge>{member.teamRole}</Badge>
                            {member.hasScope ? <Badge variant="success">scoped</Badge> : null}
                            {role === "org_owner" &&
                            member.orgRole !== "org_owner" &&
                            member.teamRole !== "team_admin" ? (
                              <Button
                                variant="ghost"
                                size="sm"
                                disabled={promotingAdminID === member.userId}
                                onClick={() => void submitPromoteToAdmin(member.userId)}
                              >
                                {promotingAdminID === member.userId ? "Assigning..." : "Make Admin"}
                              </Button>
                            ) : null}
                          </div>
                        </div>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-[#666]">No team members found.</p>
                  )}
                </div>
              ) : null}
            </div>

            <div className="rounded-lg bg-[#0a0a0a] p-4">
              <h3 className="mb-3 font-medium text-white">Team Admins</h3>
              {detailLoading ? <p className="text-sm text-[#888]">Loading admins...</p> : null}
              {detailError ? <p className="text-sm text-red-400">{detailError}</p> : null}
              {!detailLoading && !detailError ? (
                <div className="space-y-2">
                  {teamAdmins.length > 0 ? (
                    teamAdmins.map((admin) => (
                      <div key={admin.userId} className="rounded-lg border border-[#1f1f1f] px-3 py-2">
                        <p className="text-sm text-white">{admin.name || admin.email}</p>
                        <p className="text-xs text-[#888]">{admin.email}</p>
                        <p className="mt-1 font-mono text-[11px] text-[#666]">{admin.userId}</p>
                      </div>
                    ))
                  ) : (
                    <p className="text-sm text-[#666]">No team admins found.</p>
                  )}
                </div>
              ) : null}
            </div>
          </div>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-white">Teams</h2>
        {role === "org_owner" ? (
          <div className="flex items-center gap-2">
            <input
              type="text"
              value={newTeamName}
              onChange={(event) => setNewTeamName(event.target.value)}
              placeholder="new-team"
              className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
            />
            <Button variant="primary" size="sm" onClick={() => void submitCreateTeam()} disabled={creating}>
              <Plus className="h-4 w-4" />
              {creating ? "Creating..." : "Add Team"}
            </Button>
          </div>
        ) : null}
      </div>

      {createError ? <p className="text-sm text-red-400">{createError}</p> : null}

      {loading && teams.length === 0 ? <p className="text-sm text-[#888]">Loading teams...</p> : null}
      {error && teams.length === 0 ? <p className="text-sm text-red-400">{error}</p> : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        {teams.map((team) => (
          <Card
            key={team.id}
            className="cursor-pointer transition-all hover:border-[#3a3a3a]"
            onClick={() => setSelectedTeam(team)}
          >
            <div className="mb-4 flex items-start justify-between">
              <div>
                <h3 className="mb-1 font-semibold text-white">{team.name}</h3>
                <p className="text-sm text-[#666]">
                  {team.memberCount} members · {team.adminCount} admins
                </p>
              </div>
              <Users className="h-5 w-5 text-[#888]" />
            </div>
            <div className="flex flex-wrap gap-2">
              {team.policies.slice(0, 2).map((policy) => (
                <Badge key={policy}>{policy}</Badge>
              ))}
              {team.policies.length > 2 ? <Badge>+{team.policies.length - 2}</Badge> : null}
            </div>
          </Card>
        ))}
      </div>

      {!loading && teams.length === 0 ? (
        <EmptyState
          icon={<Users className="h-8 w-8 text-[#888]" />}
          title="No Teams Found"
          description="Create a team first to manage access and policies."
        />
      ) : null}
    </div>
  );
}

function ApiKeysTab({
  apiKeys,
  teams,
  loading,
  error,
  creating,
  revokingKeyID,
  defaultUserID,
  onCreateApiKey,
  onRevokeApiKey
}: {
  apiKeys: ApiKey[];
  teams: Team[];
  loading: boolean;
  error: string | null;
  creating: boolean;
  revokingKeyID: string | null;
  defaultUserID: string;
  onCreateApiKey: (teamID: string, userID: string) => Promise<string | null>;
  onRevokeApiKey: (keyID: string) => Promise<void>;
}) {
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedTeamID, setSelectedTeamID] = useState("");
  const [targetUserID, setTargetUserID] = useState(defaultUserID);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  useEffect(() => {
    if (!selectedTeamID && teams.length > 0) {
      setSelectedTeamID(teams[0].id);
    }
  }, [selectedTeamID, teams]);

  useEffect(() => {
    setTargetUserID(defaultUserID);
  }, [defaultUserID]);

  const filteredKeys = useMemo(
    () =>
      apiKeys.filter(
        (key) =>
          key.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
          key.key.toLowerCase().includes(searchTerm.toLowerCase())
      ),
    [apiKeys, searchTerm]
  );

  const submitCreateKey = async () => {
    if (!selectedTeamID) {
      setActionError("Select a team first.");
      return;
    }
    if (!targetUserID.trim()) {
      setActionError("Target user_id is required.");
      return;
    }
    setActionError(null);
    setCreatedKey(null);
    try {
      const plaintext = await onCreateApiKey(selectedTeamID, targetUserID.trim());
      if (plaintext) {
        setCreatedKey(plaintext);
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to create API key.");
    }
  };

  const submitRevokeKey = async (keyID: string) => {
    setActionError(null);
    try {
      await onRevokeApiKey(keyID);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to revoke API key.");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col items-start justify-between gap-4 md:flex-row md:items-center">
        <h2 className="text-xl font-semibold text-white">API Keys</h2>
        <div className="flex w-full items-center gap-3 md:w-auto">
          <div className="relative flex-1 md:flex-initial">
            <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-[#666]" />
            <input
              type="text"
              placeholder="Search keys..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="w-full rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] py-2 pl-10 pr-4 text-sm text-white focus:border-[#3a3a3a] focus:outline-none md:w-64"
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <select
              value={selectedTeamID}
              onChange={(event) => setSelectedTeamID(event.target.value)}
              className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
            >
              <option value="">Select team</option>
              {teams.map((team) => (
                <option key={team.id} value={team.id}>
                  {team.name}
                </option>
              ))}
            </select>
            <input
              type="text"
              value={targetUserID}
              onChange={(event) => setTargetUserID(event.target.value)}
              placeholder="target user_id"
              className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
            />
            <Button variant="primary" size="sm" onClick={() => void submitCreateKey()} disabled={creating}>
              <Plus className="h-4 w-4" />
              {creating ? "Creating..." : "Create Key"}
            </Button>
          </div>
        </div>
      </div>

      {createdKey ? (
        <p className="rounded-lg border border-green-500/30 bg-green-500/5 px-3 py-2 text-xs text-green-300">
          New API key (copy now): <span className="font-mono">{createdKey}</span>
        </p>
      ) : null}
      {actionError ? <p className="text-sm text-red-400">{actionError}</p> : null}

      {loading && apiKeys.length === 0 ? <p className="text-sm text-[#888]">Loading API keys...</p> : null}
      {error && apiKeys.length === 0 ? <p className="text-sm text-red-400">{error}</p> : null}

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[#2a2a2a]">
                <th className="px-4 py-3 text-left text-sm font-medium text-[#888]">Name</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-[#888]">Key</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-[#888]">Team</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-[#888]">Created</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-[#888]">Status</th>
                <th className="px-4 py-3 text-left text-sm font-medium text-[#888]">Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredKeys.map((key) => {
                const team = teams.find((entry) => entry.id === key.teamId);
                return (
                  <tr key={key.id} className="border-b border-[#2a2a2a] hover:bg-[#1a1a1a]">
                    <td className="px-4 py-3 text-sm text-white">{key.name}</td>
                    <td className="px-4 py-3 font-mono text-sm text-[#888]">{key.key}</td>
                    <td className="px-4 py-3 text-sm text-white">{team?.name ?? key.teamId}</td>
                    <td className="px-4 py-3 text-sm text-[#888]">{key.createdAt.toLocaleDateString()}</td>
                    <td className="px-4 py-3">
                      <Badge variant={key.revoked ? "error" : "success"}>
                        {key.revoked ? "Revoked" : "Active"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      {!key.revoked ? (
                        <Button
                          variant="ghost"
                          size="sm"
                          disabled={revokingKeyID === key.id}
                          onClick={() => void submitRevokeKey(key.id)}
                        >
                          {revokingKeyID === key.id ? "Revoking..." : "Revoke"}
                        </Button>
                      ) : (
                        <span className="text-xs text-[#666]">-</span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </Card>

      {!loading && filteredKeys.length === 0 ? (
        <EmptyState
          icon={<Key className="h-8 w-8 text-[#888]" />}
          title="No API Keys"
          description="Create keys from this page and they will appear here."
        />
      ) : null}
    </div>
  );
}

function ModelsTab({
  role,
  orgID,
  orgPolicies,
  providerKeys,
  teams,
  loading,
  error,
  writingProvider,
  writingOrgPolicy,
  writingTeamPolicy,
  onCreateProviderKey,
  onUpsertOrgPolicies,
  onUpsertTeamPolicies
}: {
  role: Role;
  orgID: string;
  orgPolicies: PolicyRow[];
  providerKeys: ProviderKey[];
  teams: Team[];
  loading: boolean;
  error: string | null;
  writingProvider: boolean;
  writingOrgPolicy: boolean;
  writingTeamPolicy: boolean;
  onCreateProviderKey: (provider: string, apiKey: string, keyKekID: string) => Promise<void>;
  onUpsertOrgPolicies: (entries: PolicyRow[]) => Promise<void>;
  onUpsertTeamPolicies: (teamID: string, entries: PolicyRow[]) => Promise<void>;
}) {
  const allowlist = orgPolicies
    .filter((policy) => policy.is_allowed)
    .map((policy) => `${policy.provider}/${policy.model}`);
  const [provider, setProvider] = useState<string>(routerSupportedProviders[0]);
  const [providerAPIKey, setProviderAPIKey] = useState("");
  const [providerKekID, setProviderKekID] = useState("kek-v1");
  const [orgPolicyJSON, setOrgPolicyJSON] = useState("[]");
  const [teamPolicyJSON, setTeamPolicyJSON] = useState("");
  const [policyTeamID, setPolicyTeamID] = useState("");
  const [actionError, setActionError] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);

  useEffect(() => {
    setOrgPolicyJSON(JSON.stringify(orgPolicies, null, 2));
  }, [orgPolicies]);

  useEffect(() => {
    if (!policyTeamID && teams.length > 0) {
      setPolicyTeamID(teams[0].id);
      return;
    }

    if (!policyTeamID || !orgID) {
      setTeamPolicyJSON("");
      return;
    }

    let cancelled = false;
    const loadTeamPolicies = async () => {
      try {
        const teamPoliciesRes = await getJSON<ListResponse<PolicyRow>>(
          `/v1/control/orgs/${encodeURIComponent(orgID)}/teams/${encodeURIComponent(policyTeamID)}/policies/models`
        );
        if (cancelled) {
          return;
        }
        setTeamPolicyJSON(JSON.stringify(toArray(teamPoliciesRes.items), null, 2));
      } catch {
        if (cancelled) {
          return;
        }
        setTeamPolicyJSON("[]");
      }
    };
    void loadTeamPolicies();

    return () => {
      cancelled = true;
    };
  }, [orgID, policyTeamID, teams]);

  const submitProviderKey = async () => {
    if (!orgID) {
      setActionError("Missing org session context.");
      return;
    }
    if (!providerAPIKey.trim()) {
      setActionError("API key is required.");
      return;
    }
    setActionError(null);
    setActionMessage(null);
    try {
      await onCreateProviderKey(provider, providerAPIKey.trim(), providerKekID.trim() || "kek-v1");
      setProviderAPIKey("");
      setActionMessage(`Stored provider key for ${provider}.`);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to store provider key.");
    }
  };

  const submitOrgPolicies = async () => {
    if (!orgID) {
      setActionError("Missing org session context.");
      return;
    }
    let entries: PolicyRow[];
    try {
      entries = JSON.parse(orgPolicyJSON) as PolicyRow[];
    } catch {
      setActionError("Org policy JSON is invalid.");
      return;
    }
    setActionError(null);
    setActionMessage(null);
    try {
      await onUpsertOrgPolicies(entries);
      setActionMessage(`Updated ${entries.length} org policy entries.`);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to update org policies.");
    }
  };

  const submitTeamPolicies = async () => {
    if (!orgID) {
      setActionError("Missing org session context.");
      return;
    }
    if (!policyTeamID) {
      setActionError("Select a team first.");
      return;
    }
    if (!teamPolicyJSON.trim()) {
      setActionError("Team policy JSON is empty.");
      return;
    }
    let entries: PolicyRow[];
    try {
      entries = JSON.parse(teamPolicyJSON) as PolicyRow[];
    } catch {
      setActionError("Team policy JSON is invalid.");
      return;
    }
    setActionError(null);
    setActionMessage(null);
    try {
      await onUpsertTeamPolicies(policyTeamID, entries);
      setActionMessage(`Updated ${entries.length} team policy entries.`);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "Failed to update team policies.");
    }
  };

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold text-white">Models & Policies</h2>

      {loading && orgPolicies.length === 0 ? <p className="text-sm text-[#888]">Loading policies...</p> : null}
      {error && orgPolicies.length === 0 ? <p className="text-sm text-red-400">{error}</p> : null}
      {actionMessage ? <p className="text-sm text-green-400">{actionMessage}</p> : null}
      {actionError ? <p className="text-sm text-red-400">{actionError}</p> : null}

      <Card>
        <h3 className="mb-2 font-semibold text-white">Model Provider Keys</h3>
        <p className="mb-4 text-sm text-[#888]">
          Router completion adapters currently support: {routerSupportedProviders.join(", ")}.
          Add org provider keys before enabling model policies.
        </p>
        {role === "org_owner" ? (
          <div className="mb-4 grid grid-cols-1 gap-3 md:grid-cols-4">
            <select
              value={provider}
              onChange={(event) => setProvider(event.target.value)}
              className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
            >
              {routerSupportedProviders.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
            <input
              type="password"
              value={providerAPIKey}
              onChange={(event) => setProviderAPIKey(event.target.value)}
              placeholder="provider api key"
              className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
            />
            <input
              type="text"
              value={providerKekID}
              onChange={(event) => setProviderKekID(event.target.value)}
              placeholder="key kek id"
              className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
            />
            <Button variant="primary" onClick={() => void submitProviderKey()} disabled={writingProvider}>
              {writingProvider ? "Storing..." : "Store Provider Key"}
            </Button>
          </div>
        ) : (
          <p className="mb-4 text-sm text-[#666]">Only org owner can store provider keys.</p>
        )}
        <div className="space-y-2">
          {providerKeys.length > 0 ? (
            providerKeys.map((entry) => (
              <div key={entry.id} className="flex items-center justify-between rounded-lg bg-[#0a0a0a] p-3">
                <div className="text-sm text-white">
                  {entry.provider} · {entry.keyKekID}
                </div>
                <div className="flex items-center gap-2 text-xs text-[#888]">
                  <Badge variant={entry.isActive ? "success" : "warning"}>
                    {entry.isActive ? "active" : "inactive"}
                  </Badge>
                  <span>{entry.createdAt.toLocaleString()}</span>
                </div>
              </div>
            ))
          ) : (
            <p className="text-sm text-[#666]">No provider keys stored yet.</p>
          )}
        </div>
      </Card>

      <Card>
        <h3 className="mb-4 font-semibold text-white">Organization Policies</h3>
        <p className="mb-4 text-sm text-[#888]">Current org-level model policy rows. Edit JSON and upsert.</p>
        <div className="flex flex-wrap gap-2">
          {orgPolicies.length > 0 ? (
            orgPolicies.map((entry) => (
              <Badge key={`${entry.provider}:${entry.model}`} variant={entry.is_allowed ? "success" : "warning"}>
                {entry.provider}/{entry.model} {entry.is_allowed ? "allow" : "deny"}
              </Badge>
            ))
          ) : (
            <span className="text-sm text-[#666]">No policy rows configured.</span>
          )}
        </div>
        {role === "org_owner" ? (
          <div className="mt-4 space-y-3">
            <textarea
              rows={8}
              value={orgPolicyJSON}
              onChange={(event) => setOrgPolicyJSON(event.target.value)}
              className="w-full rounded-lg border border-[#2a2a2a] bg-[#0f0f0f] p-3 font-mono text-xs text-white focus:border-[#3a3a3a] focus:outline-none"
            />
            <Button variant="primary" onClick={() => void submitOrgPolicies()} disabled={writingOrgPolicy}>
              {writingOrgPolicy ? "Updating..." : "Upsert Org Policies"}
            </Button>
          </div>
        ) : null}
      </Card>

      <Card>
        <h3 className="mb-4 font-semibold text-white">Team Policies & Effective Models</h3>
        {teams.length > 0 ? (
          <div className="mb-4 space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <select
                value={policyTeamID}
                onChange={(event) => setPolicyTeamID(event.target.value)}
                className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-2 text-sm text-white focus:border-[#3a3a3a] focus:outline-none"
              >
                {teams.map((team) => (
                  <option key={team.id} value={team.id}>
                    {team.name}
                  </option>
                ))}
              </select>
              <Button variant="primary" onClick={() => void submitTeamPolicies()} disabled={writingTeamPolicy}>
                {writingTeamPolicy ? "Updating..." : "Upsert Team Policies"}
              </Button>
            </div>
            <textarea
              rows={8}
              value={teamPolicyJSON}
              onChange={(event) => setTeamPolicyJSON(event.target.value)}
              className="w-full rounded-lg border border-[#2a2a2a] bg-[#0f0f0f] p-3 font-mono text-xs text-white focus:border-[#3a3a3a] focus:outline-none"
            />
          </div>
        ) : null}
        <div className="space-y-3">
          {teams.map((team) => (
            <div key={team.id} className="rounded-lg bg-[#0a0a0a] p-4">
              <div className="mb-2 flex items-center justify-between">
                <h4 className="font-medium text-white">{team.name}</h4>
                <Badge>{team.id}</Badge>
              </div>
              <div className="flex flex-wrap gap-2">
                {team.models.length > 0 ? (
                  team.models.map((model) => <Badge key={`${team.id}:${model}`}>{model}</Badge>)
                ) : (
                  <span className="text-sm text-[#666]">No effective models.</span>
                )}
              </div>
              {allowlist.length > 0 ? null : (
                <p className="mt-2 text-xs text-[#666]">Org allowlist is empty, so effective set can be empty.</p>
              )}
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

function UsageTab({
  role,
  dateRange,
  setDateRange,
  bucket,
  setBucket,
  usageData,
  timelineBuckets,
  usageByTeam,
  usageByModel,
  loading,
  error
}: {
  role: Role;
  dateRange: string;
  setDateRange: (value: string) => void;
  bucket: string;
  setBucket: (value: string) => void;
  usageData: UsageData;
  timelineBuckets: number[];
  usageByTeam: Array<{ name: string; value: number }>;
  usageByModel: Array<{ name: string; value: number }>;
  loading: boolean;
  error: string | null;
}) {
  if (role !== "org_owner") {
    return (
      <EmptyState
        icon={<Activity className="h-8 w-8 text-[#888]" />}
        title="Usage Available to Org Admin"
        description="Usage analytics and metrics are only available to organization owners."
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col items-start justify-between gap-4 md:flex-row md:items-center">
        <h2 className="text-xl font-semibold text-white">Usage Analytics</h2>
        <div className="flex items-center gap-3">
          <select
            value={dateRange}
            onChange={(e) => setDateRange(e.target.value)}
            className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-4 py-2 text-sm text-white focus:outline-none"
          >
            <option value="24h">Last 24 hours</option>
            <option value="7d">Last 7 days</option>
            <option value="30d">Last 30 days</option>
            <option value="90d">Last 90 days</option>
          </select>
          <select
            value={bucket}
            onChange={(e) => setBucket(e.target.value)}
            className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-4 py-2 text-sm text-white focus:outline-none"
          >
            <option value="hour">Hourly</option>
            <option value="day">Daily</option>
          </select>
          <Button variant="secondary" size="sm" disabled>
            <Download className="h-4 w-4" />
          </Button>
        </div>
      </div>

      {loading ? <p className="text-sm text-[#888]">Loading usage...</p> : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      <div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
        <KPICard
          title="Total Requests"
          value={usageData.requests.toLocaleString()}
          icon={<Activity className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Input Tokens"
          value={`${(usageData.inputTokens / 1000000).toFixed(2)}M`}
          icon={<BarChart3 className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Output Tokens"
          value={`${(usageData.outputTokens / 1000000).toFixed(2)}M`}
          icon={<BarChart3 className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Error Rate"
          value={`${(usageData.errorRate * 100).toFixed(2)}%`}
          icon={<XCircle className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="P50 Latency"
          value={`${Math.round(usageData.p50Latency)}ms`}
          icon={<Zap className="h-5 w-5 text-white" />}
        />
        <KPICard
          title="Estimated Cost"
          value={`$${usageData.estimatedCost.toFixed(2)}`}
          icon={<TrendingUp className="h-5 w-5 text-white" />}
          subtitle="Heuristic token pricing"
        />
      </div>

      <Card>
        <h3 className="mb-4 font-semibold text-white">Request Timeline</h3>
        {timelineBuckets.length > 0 ? (
          <div className="flex h-64 items-end justify-between gap-2">
            {timelineBuckets.map((height, index) => (
              <div
                key={index}
                className="flex-1 rounded-t bg-gradient-to-t from-white to-[#888] opacity-70 transition-opacity hover:opacity-100"
                style={{ height: `${height}%` }}
              />
            ))}
          </div>
        ) : (
          <p className="text-sm text-[#666]">No timeline data in selected range.</p>
        )}
      </Card>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <h3 className="mb-4 font-semibold text-white">Usage by Team</h3>
          <div className="space-y-3">
            {usageByTeam.length > 0 ? (
              usageByTeam.map((entry) => (
                <div key={entry.name}>
                  <div className="mb-1 flex items-center justify-between">
                    <span className="text-sm text-white">{entry.name}</span>
                    <span className="text-sm text-[#888]">{entry.value.toFixed(1)}%</span>
                  </div>
                  <div className="h-2 w-full rounded-full bg-[#2a2a2a]">
                    <div className="h-2 rounded-full bg-white" style={{ width: `${entry.value}%` }} />
                  </div>
                </div>
              ))
            ) : (
              <p className="text-sm text-[#666]">No team usage data.</p>
            )}
          </div>
        </Card>

        <Card>
          <h3 className="mb-4 font-semibold text-white">Usage by Model</h3>
          <div className="space-y-3">
            {usageByModel.length > 0 ? (
              usageByModel.map((entry) => (
                <div key={entry.name}>
                  <div className="mb-1 flex items-center justify-between">
                    <span className="text-sm text-white">{entry.name}</span>
                    <span className="text-sm text-[#888]">{entry.value.toFixed(1)}%</span>
                  </div>
                  <div className="h-2 w-full rounded-full bg-[#2a2a2a]">
                    <div className="h-2 rounded-full bg-white" style={{ width: `${entry.value}%` }} />
                  </div>
                </div>
              ))
            ) : (
              <p className="text-sm text-[#666]">No model usage data.</p>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}

function EventsTab({ role, events, loading, error }: { role: Role; events: EventItem[]; loading: boolean; error: string | null }) {
  if (role !== "org_owner") {
    return (
      <EmptyState
        icon={<FileText className="h-8 w-8 text-[#888]" />}
        title="Events Available to Org Admin"
        description="Event logs and audit trails are only available to organization owners."
      />
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-white">Event Log</h2>
        <Button variant="secondary" size="sm" disabled>
          <Download className="h-4 w-4" />
          Export
        </Button>
      </div>

      {loading && events.length === 0 ? <p className="text-sm text-[#888]">Loading events...</p> : null}
      {error && events.length === 0 ? <p className="text-sm text-red-400">{error}</p> : null}

      <Card>
        <div className="space-y-4">
          {events.map((event) => (
            <div
              key={event.id}
              className="flex items-start gap-4 rounded-lg bg-[#0a0a0a] p-4 transition-colors hover:bg-[#1a1a1a]"
            >
              <div className="mt-1">
                {event.status === "success" ? (
                  <CheckCircle className="h-5 w-5 text-green-500" />
                ) : event.status === "warning" ? (
                  <AlertCircle className="h-5 w-5 text-yellow-500" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-500" />
                )}
              </div>
              <div className="flex-1">
                <p className="mb-1 text-sm text-white">{event.description}</p>
                <div className="flex items-center gap-3 text-xs text-[#666]">
                  <span>{event.user}</span>
                  <span>•</span>
                  <span>{event.timestamp.toLocaleString()}</span>
                </div>
              </div>
              <Badge>{event.type}</Badge>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

export function BaseConsole() {
  const [activeTab, setActiveTab] = useState("overview");
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  const [session, setSession] = useState<SessionResponse | null>(null);
  const [sessionError, setSessionError] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [summary, setSummary] = useState<SummaryResponse | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [apiKeys, setAPIKeys] = useState<ApiKey[]>([]);
  const [orgPolicies, setOrgPolicies] = useState<PolicyRow[]>([]);
  const [providerKeys, setProviderKeys] = useState<ProviderKey[]>([]);
  const [usageData, setUsageData] = useState<UsageData>(emptyUsageData);
  const [timelineBuckets, setTimelineBuckets] = useState<number[]>([]);
  const [usageByTeam, setUsageByTeam] = useState<Array<{ name: string; value: number }>>([]);
  const [usageByModel, setUsageByModel] = useState<Array<{ name: string; value: number }>>([]);
  const [events, setEvents] = useState<EventItem[]>([]);

  const [creatingTeam, setCreatingTeam] = useState(false);
  const [creatingAPIKey, setCreatingAPIKey] = useState(false);
  const [revokingKeyID, setRevokingKeyID] = useState<string | null>(null);
  const [writingProvider, setWritingProvider] = useState(false);
  const [writingOrgPolicy, setWritingOrgPolicy] = useState(false);
  const [writingTeamPolicy, setWritingTeamPolicy] = useState(false);

  const [dateRange, setDateRange] = useState("7d");
  const [bucket, setBucket] = useState("day");

  const role: Role = session?.role ?? "member";

  const refreshSession = useCallback(async () => {
    try {
      const currentSession = await getJSON<SessionResponse>("/v1/control/session");
      setSession(currentSession);
      setSessionError(null);
    } catch (error) {
      setSession(null);
      setSessionError(error instanceof Error ? error.message : "Unable to fetch session.");
    }
  }, []);

  const refreshData = useCallback(async () => {
    if (!session) {
      setSummary(null);
      setTeams([]);
      setAPIKeys([]);
      setOrgPolicies([]);
      setProviderKeys([]);
      setUsageData(emptyUsageData);
      setTimelineBuckets([]);
      setUsageByTeam([]);
      setUsageByModel([]);
      setEvents([]);
      return;
    }

    setLoading(true);
    setLoadError(null);

    try {
      const orgID = encodeURIComponent(session.org_id);

      const summaryRes = await getJSON<SummaryResponse>(`/v1/control/orgs/${orgID}/summary`);
      setSummary(summaryRes);

      const teamsRes = await getJSON<ListResponse<TeamListItem>>(`/v1/control/orgs/${orgID}/teams?limit=200`);
      const teamRows = toArray(teamsRes.items);

      const resolvedTeams = teamRows.map((team) => ({
        id: team.id,
        name: team.name,
        memberCount: 0,
        adminCount: 0,
        policies: [],
        models: []
      }));
      setTeams(resolvedTeams);

      const orgPoliciesRes = await getJSON<ListResponse<PolicyRow>>(
        `/v1/control/orgs/${orgID}/policies/models`
      );
      setOrgPolicies(toArray(orgPoliciesRes.items));

      if (role === "org_owner") {
        const providerKeysRes = await getJSON<
          ListResponse<{
            id: string;
            provider: string;
            key_kek_id: string;
            is_active: boolean;
            created_at: string;
          }>
        >(`/v1/control/orgs/${orgID}/providers/keys`);
        setProviderKeys(
          toArray(providerKeysRes.items).map((entry) => ({
            id: entry.id,
            provider: entry.provider,
            keyKekID: entry.key_kek_id,
            isActive: entry.is_active,
            createdAt: new Date(entry.created_at)
          }))
        );

        const apiKeysRes = await getJSON<
          ListResponse<{
            id: string;
            key_prefix: string;
            user_id: string;
            team_id: string;
            is_active: boolean;
            created_at: string;
            last_used_at?: string | null;
          }>
        >(`/v1/control/orgs/${orgID}/api-keys?include_revoked=true&limit=200`);
        setAPIKeys(
          toArray(apiKeysRes.items).map((entry) => ({
            id: entry.id,
            name: entry.key_prefix,
            key: `${entry.key_prefix}...`,
            teamId: entry.team_id,
            userId: entry.user_id,
            createdAt: new Date(entry.created_at),
            lastUsed: entry.last_used_at ? new Date(entry.last_used_at) : undefined,
            revoked: !entry.is_active
          }))
        );

        const now = new Date();
        const from = fromForRange(dateRange, now);
        const usageQuery = new URLSearchParams({
          from: from.toISOString(),
          to: now.toISOString()
        }).toString();

        const [usageSummaryRes, usageSeriesRes, usageByTeamRes, usageByModelRes] = await Promise.all([
          getJSON<UsageSummaryResponse>(`/v1/control/orgs/${orgID}/usage/summary?${usageQuery}`),
          getJSON<ListResponse<{ request_count: number }>>(
            `/v1/control/orgs/${orgID}/usage/timeseries?${usageQuery}&bucket=${encodeURIComponent(bucket)}`
          ),
          getJSON<ListResponse<{ team_id: string; request_count: number }>>(
            `/v1/control/orgs/${orgID}/usage/by-team?${usageQuery}`
          ),
          getJSON<ListResponse<{ provider: string; model: string; request_count: number }>>(
            `/v1/control/orgs/${orgID}/usage/by-model?${usageQuery}`
          )
        ]);

        const estimatedCost =
          (usageSummaryRes.request_tokens / 1_000_000) * 5 +
          (usageSummaryRes.response_tokens / 1_000_000) * 15;

        setUsageData({
          requests: usageSummaryRes.request_count,
          inputTokens: usageSummaryRes.request_tokens,
          outputTokens: usageSummaryRes.response_tokens,
          errorRate: usageSummaryRes.error_rate,
          p50Latency: usageSummaryRes.latency_p50_ms,
          p95Latency: usageSummaryRes.latency_p95_ms,
          estimatedCost
        });

        const seriesItems = toArray(usageSeriesRes.items);
        const maxSeries = Math.max(
          1,
          ...seriesItems.map((entry) => Number(entry.request_count) || 0)
        );
        setTimelineBuckets(
          seriesItems.map((entry) => {
            const value = Number(entry.request_count) || 0;
            return Math.max(6, Math.round((value / maxSeries) * 100));
          })
        );

        const usageTeamItems = toArray(usageByTeamRes.items);
        const usageTeamTotal = usageTeamItems.reduce(
          (acc, entry) => acc + (Number(entry.request_count) || 0),
          0
        );
        const teamNameByID = new Map(resolvedTeams.map((entry) => [entry.id, entry.name]));
        setUsageByTeam(
          usageTeamItems.map((entry) => ({
            name: teamNameByID.get(entry.team_id) ?? entry.team_id,
            value:
              usageTeamTotal > 0 ? ((Number(entry.request_count) || 0) / usageTeamTotal) * 100 : 0
          }))
        );

        const usageModelItems = toArray(usageByModelRes.items);
        const usageModelTotal = usageModelItems.reduce(
          (acc, entry) => acc + (Number(entry.request_count) || 0),
          0
        );
        setUsageByModel(
          usageModelItems.map((entry) => ({
            name: `${entry.provider}/${entry.model}`,
            value:
              usageModelTotal > 0 ? ((Number(entry.request_count) || 0) / usageModelTotal) * 100 : 0
          }))
        );

        const eventsRes = await getJSON<
          ListResponse<{
            id: string;
            type: string;
            team_id: string | null;
            user_id: string | null;
            resource_id: string;
            occurred_at: string;
          }>
        >(`/v1/control/orgs/${orgID}/events?limit=100`);

        setEvents(
          toArray(eventsRes.items).map((entry) => ({
            id: entry.id,
            type: entry.type,
            user: entry.user_id ?? "system",
            description: describeEvent(entry.type, entry.team_id, entry.user_id, entry.resource_id),
            timestamp: new Date(entry.occurred_at),
            status: eventStatus(entry.type)
          }))
        );
      } else {
        setAPIKeys([]);
        setProviderKeys([]);
        setUsageData(emptyUsageData);
        setTimelineBuckets([]);
        setUsageByTeam([]);
        setUsageByModel([]);
        setEvents([]);
      }
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Unable to load console data.");
    } finally {
      setLoading(false);
    }
  }, [bucket, dateRange, role, session]);

  const createTeam = useCallback(
    async (name: string) => {
      if (!session?.org_id) {
        throw new Error("Missing org session context.");
      }
      setCreatingTeam(true);
      try {
        await postJSON(`/v1/control/orgs/${encodeURIComponent(session.org_id)}/teams`, { name });
        await refreshData();
      } finally {
        setCreatingTeam(false);
      }
    },
    [refreshData, session?.org_id]
  );

  const createApiKey = useCallback(
    async (teamID: string, userID: string): Promise<string | null> => {
      if (!session?.org_id) {
        throw new Error("Missing org session context.");
      }
      setCreatingAPIKey(true);
      try {
        const result = await postJSON<{ api_key?: string }>(
          `/v1/control/orgs/${encodeURIComponent(session.org_id)}/teams/${encodeURIComponent(teamID)}/users/${encodeURIComponent(userID)}/api-keys`,
          {}
        );
        await refreshData();
        return typeof result.api_key === "string" ? result.api_key : null;
      } finally {
        setCreatingAPIKey(false);
      }
    },
    [refreshData, session?.org_id]
  );

  const revokeApiKey = useCallback(
    async (keyID: string) => {
      if (!session?.org_id) {
        throw new Error("Missing org session context.");
      }
      setRevokingKeyID(keyID);
      try {
        await postJSON(`/v1/control/orgs/${encodeURIComponent(session.org_id)}/api-keys/${encodeURIComponent(keyID)}/revoke`, {});
        await refreshData();
      } finally {
        setRevokingKeyID(null);
      }
    },
    [refreshData, session?.org_id]
  );

  const createProviderKey = useCallback(
    async (provider: string, apiKey: string, keyKekID: string) => {
      if (!session?.org_id) {
        throw new Error("Missing org session context.");
      }
      setWritingProvider(true);
      try {
        await postJSON(
          `/v1/control/orgs/${encodeURIComponent(session.org_id)}/providers/${encodeURIComponent(provider)}/keys`,
          {
            api_key: apiKey,
            key_kek_id: keyKekID
          }
        );
        await refreshData();
      } finally {
        setWritingProvider(false);
      }
    },
    [refreshData, session?.org_id]
  );

  const upsertOrgPolicies = useCallback(
    async (entries: PolicyRow[]) => {
      if (!session?.org_id) {
        throw new Error("Missing org session context.");
      }
      setWritingOrgPolicy(true);
      try {
        await putJSON(`/v1/control/orgs/${encodeURIComponent(session.org_id)}/policies/models`, { entries });
        await refreshData();
      } finally {
        setWritingOrgPolicy(false);
      }
    },
    [refreshData, session?.org_id]
  );

  const upsertTeamPolicies = useCallback(
    async (teamID: string, entries: PolicyRow[]) => {
      if (!session?.org_id) {
        throw new Error("Missing org session context.");
      }
      setWritingTeamPolicy(true);
      try {
        await putJSON(
          `/v1/control/orgs/${encodeURIComponent(session.org_id)}/teams/${encodeURIComponent(teamID)}/policies/models`,
          { entries }
        );
        await refreshData();
      } finally {
        setWritingTeamPolicy(false);
      }
    },
    [refreshData, session?.org_id]
  );

  const refreshAll = useCallback(async () => {
    await refreshSession();
    await refreshData();
  }, [refreshData, refreshSession]);

  useEffect(() => {
    void refreshSession();
  }, [refreshSession]);

  useEffect(() => {
    void refreshData();
  }, [refreshData]);

  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white">
      <header className="sticky top-0 z-40 border-b border-[#2a2a2a] bg-[#0a0a0a]">
        <div className="mx-auto h-16 max-w-7xl px-4 sm:px-6 lg:px-8">
          <div className="flex h-full items-center justify-between">
            <div className="flex items-center gap-8">
              <div className="flex items-center gap-2">
                <Layers className="h-6 w-6 text-white" />
                <span className="text-lg font-semibold">Admin Console</span>
              </div>
              <Navigation
                activeTab={activeTab}
                setActiveTab={setActiveTab}
                role={role}
                isMobileMenuOpen={isMobileMenuOpen}
                setIsMobileMenuOpen={setIsMobileMenuOpen}
              />
            </div>

            <div className="flex items-center gap-3">
              {session ? (
                <span className="rounded-lg border border-[#2a2a2a] bg-[#1a1a1a] px-3 py-1.5 text-sm text-white">
                  role: {session.role}
                </span>
              ) : null}
              <Button variant="ghost" size="sm" onClick={() => void refreshAll()}>
                <Bell className="h-4 w-4" />
              </Button>
              <Button variant="ghost" size="sm" onClick={() => void refreshAll()}>
                <Settings className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        {session ? (
          <div className="mb-4 rounded-lg border border-[#2a2a2a] bg-[#111] px-4 py-3 text-xs text-[#888]">
            org_id: <span className="font-mono text-[#ccc]">{session.org_id}</span> · user_id: {" "}
            <span className="font-mono text-[#ccc]">{session.user_id}</span> · role: {" "}
            <span className="font-mono text-[#ccc]">{session.role}</span>
          </div>
        ) : (
          <div className="mb-6 rounded-lg border border-yellow-500/30 bg-yellow-500/5 p-4">
            <p className="text-sm text-yellow-300">
              {sessionError ?? "No active control session."} Sign in from home and come back here.
            </p>
            <Link href="/" className="mt-2 inline-flex text-sm text-white underline">
              Back to home
            </Link>
          </div>
        )}

        <AnimatePresence mode="wait">
          <motion.div
            key={activeTab}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -20 }}
            transition={{ duration: 0.2 }}
          >
            {activeTab === "overview" ? (
              <OverviewTab
                role={role}
                loading={loading}
                error={loadError}
                summary={summary}
                teams={teams}
                events={events}
              />
            ) : null}
            {activeTab === "teams" ? (
              <TeamsTab
                role={role}
                orgID={session?.org_id ?? ""}
                teams={teams}
                loading={loading}
                error={loadError}
                creating={creatingTeam}
                onCreateTeam={createTeam}
                onRefresh={refreshData}
              />
            ) : null}
            {activeTab === "api-keys" ? (
              <ApiKeysTab
                apiKeys={apiKeys}
                teams={teams}
                loading={loading}
                error={loadError}
                creating={creatingAPIKey}
                revokingKeyID={revokingKeyID}
                defaultUserID={session?.user_id ?? ""}
                onCreateApiKey={createApiKey}
                onRevokeApiKey={revokeApiKey}
              />
            ) : null}
            {activeTab === "models" ? (
              <ModelsTab
                role={role}
                orgID={session?.org_id ?? ""}
                orgPolicies={orgPolicies}
                providerKeys={providerKeys}
                teams={teams}
                loading={loading}
                error={loadError}
                writingProvider={writingProvider}
                writingOrgPolicy={writingOrgPolicy}
                writingTeamPolicy={writingTeamPolicy}
                onCreateProviderKey={createProviderKey}
                onUpsertOrgPolicies={upsertOrgPolicies}
                onUpsertTeamPolicies={upsertTeamPolicies}
              />
            ) : null}
            {activeTab === "usage" ? (
              <UsageTab
                role={role}
                dateRange={dateRange}
                setDateRange={setDateRange}
                bucket={bucket}
                setBucket={setBucket}
                usageData={usageData}
                timelineBuckets={timelineBuckets}
                usageByTeam={usageByTeam}
                usageByModel={usageByModel}
                loading={loading}
                error={loadError}
              />
            ) : null}
            {activeTab === "events" ? (
              <EventsTab role={role} events={events} loading={loading} error={loadError} />
            ) : null}
          </motion.div>
        </AnimatePresence>
      </main>
    </div>
  );
}

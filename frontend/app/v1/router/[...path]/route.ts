import { NextRequest, NextResponse } from "next/server";

import { proxyRequest } from "@/lib/proxy";

const ROUTER_API_BASE = process.env.ROUTER_API_BASE ?? "http://127.0.0.1:8081";

type Context = { params: Promise<{ path: string[] }> };

async function proxyRouter(request: NextRequest, context: Context): Promise<NextResponse> {
  const { path } = await context.params;
  return proxyRequest(request, ROUTER_API_BASE, ["v1", "router", ...path]);
}

export const runtime = "nodejs";

export function GET(request: NextRequest, context: Context): Promise<NextResponse> {
  return proxyRouter(request, context);
}

export function POST(request: NextRequest, context: Context): Promise<NextResponse> {
  return proxyRouter(request, context);
}

export function PUT(request: NextRequest, context: Context): Promise<NextResponse> {
  return proxyRouter(request, context);
}

export function PATCH(request: NextRequest, context: Context): Promise<NextResponse> {
  return proxyRouter(request, context);
}

export function DELETE(request: NextRequest, context: Context): Promise<NextResponse> {
  return proxyRouter(request, context);
}

export function OPTIONS(request: NextRequest, context: Context): Promise<NextResponse> {
  return proxyRouter(request, context);
}

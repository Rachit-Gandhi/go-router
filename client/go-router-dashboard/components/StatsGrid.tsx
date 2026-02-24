const stats = [
  { value: "30T", label: "Monthly Tokens" },
  { value: "5M+", label: "Global Users" },
  { value: "60+", label: "Active Providers" },
  { value: "300+", label: "Models" },
];

export function StatsGrid() {
  return (
    <section className="mx-auto mt-12 grid w-full max-w-4xl grid-cols-2 gap-8 px-6 text-center md:grid-cols-4">
      {stats.map((item) => (
        <div key={item.label}>
          <p className="text-4xl font-bold">{item.value}</p>
          <p className="mt-1 text-sm text-muted-foreground">{item.label}</p>
        </div>
      ))}
    </section>
  );
}

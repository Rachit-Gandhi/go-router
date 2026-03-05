export default function Dashboard() {
  return (
    <main style={{ padding: "24px", maxWidth: 900, margin: "0 auto" }}>
      <h1>API Management</h1>
      <p>Manage your API keys (create / revoke / delete).</p>

      <section style={{ marginTop: 24 }}>
        <h2>API Keys</h2>
        <div style={{ display: "grid", gap: 12 }}>
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              style={{
                border: "1px solid #2b2b2b",
                borderRadius: 12,
                padding: 16,
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
              }}
            >
              <div>
                <div style={{ fontWeight: 600 }}>Key {i}</div>
                <div style={{ opacity: 0.7, fontSize: 12 }}>go-**{i}**</div>
              </div>
              <div style={{ display: "flex", gap: 8 }}>
                <button>Revoke</button>
                <button>Delete</button>
              </div>
            </div>
          ))}
        </div>
      </section>
    </main>
  );
}

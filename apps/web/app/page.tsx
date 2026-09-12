const navigation = [
  ["Overview", "OV"],
  ["People", "PE"],
  ["Attendance", "AT"],
  ["Leave", "LV"],
  ["Payroll", "PY"],
  ["Performance", "PF"],
  ["Recruitment", "RC"],
  ["Reports", "RP"],
];

const metrics = [
  { label: "Total employees", value: "248", change: "+12 this quarter", tone: "positive" },
  { label: "Present today", value: "224", change: "90.3% attendance", tone: "neutral" },
  { label: "On leave", value: "14", change: "8 planned · 6 sick", tone: "neutral" },
  { label: "Open roles", value: "18", change: "7 in final stage", tone: "positive" },
];

const people = [
  { initials: "AM", name: "Ava Morgan", role: "Senior Product Designer", team: "Product", status: "Online" },
  { initials: "DN", name: "Daniel Ng", role: "Platform Engineer", team: "Engineering", status: "Remote" },
  { initials: "SI", name: "Sara Ibrahim", role: "People Operations Lead", team: "People", status: "Online" },
  { initials: "JL", name: "Jonas Lee", role: "Account Executive", team: "Revenue", status: "Leave" },
];

const activity = [
  ["Leave request", "Mia requested annual leave · Sep 18–20", "6m"],
  ["New hire", "Noah completed engineering onboarding", "24m"],
  ["Performance", "Q3 review cycle is 76% complete", "1h"],
  ["Recruitment", "3 candidates moved to final interview", "2h"],
];

export default function Home() {
  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">A</span><span>Advance</span></div>
        <div className="workspace"><span className="avatar company">N</span><div><strong>Northstar Labs</strong><small>Primary workspace</small></div><span className="chevron">⌄</span></div>
        <nav>{navigation.map(([label, icon], index) => <a className={index === 0 ? "nav-item active" : "nav-item"} href="#" key={label}><span className="nav-icon">{icon}</span>{label}</a>)}</nav>
        <div className="sidebar-bottom">
          <a className="nav-item" href="#"><span className="nav-icon">ST</span>Settings</a>
          <div className="profile"><span className="avatar">SK</span><div><strong>Savi</strong><small>Administrator</small></div><span className="more">•••</span></div>
        </div>
      </aside>

      <section className="content">
        <header className="topbar">
          <div className="search"><span>⌕</span><input aria-label="Search" placeholder="Search people, teams, documents..."/><kbd>⌘ K</kbd></div>
          <div className="header-actions"><button className="icon-button" aria-label="Notifications">◌<span className="notification-dot" /></button><button className="primary-button">+ Add employee</button></div>
        </header>

        <div className="page">
          <div className="page-heading"><div><p className="eyebrow">SATURDAY · SEPTEMBER 12</p><h1>Good morning, Savi.</h1><p>Here’s what’s happening across your organization.</p></div><button className="secondary-button">Export report</button></div>

          <div className="metrics-grid">{metrics.map((metric) => <article className="metric-card" key={metric.label}><div className="metric-label">{metric.label}<button aria-label={`More options for ${metric.label}`}>•••</button></div><div className="metric-value">{metric.value}</div><div className={`metric-change ${metric.tone}`}>{metric.change}</div></article>)}</div>

          <div className="dashboard-grid">
            <article className="panel attendance-panel">
              <div className="panel-heading"><div><h2>Attendance</h2><p>Today’s workforce status</p></div><button className="text-button">View attendance →</button></div>
              <div className="attendance-main"><div className="ring"><div><strong>90%</strong><span>present</span></div></div><div className="attendance-breakdown"><div><span><i className="dot present"/>Present</span><strong>224</strong></div><div><span><i className="dot remote"/>Remote</span><strong>31</strong></div><div><span><i className="dot leave"/>On leave</span><strong>14</strong></div><div><span><i className="dot absent"/>Absent</span><strong>10</strong></div></div></div>
              <div className="insight"><span className="spark">✦</span><div><strong>Workforce insight</strong><p>Attendance is 3.2% above your 30-day average. Engineering has the strongest consistency this week.</p></div></div>
            </article>

            <article className="panel leave-panel">
              <div className="panel-heading"><div><h2>Leave requests</h2><p>3 requests need your attention</p></div><button className="text-button">View all →</button></div>
              <div className="request"><span className="avatar coral">MN</span><div><strong>Mia Novak</strong><p>Annual leave · Sep 18–20 · 3 days</p></div><span className="pending">Pending</span></div>
              <div className="request"><span className="avatar blue">RK</span><div><strong>Ravi Kumar</strong><p>Remote work · Sep 16 · 1 day</p></div><span className="pending">Pending</span></div>
              <div className="request"><span className="avatar violet">ET</span><div><strong>Ella Thompson</strong><p>Annual leave · Oct 2–6 · 3 days</p></div><span className="pending">Pending</span></div>
              <button className="full-button">Review requests</button>
            </article>

            <article className="panel people-panel">
              <div className="panel-heading"><div><h2>People</h2><p>Recently active employees</p></div><button className="text-button">View directory →</button></div>
              <div className="table-head"><span>Employee</span><span>Team</span><span>Status</span></div>
              {people.map((person) => <div className="person-row" key={person.name}><div className="person"><span className="avatar soft">{person.initials}</span><div><strong>{person.name}</strong><p>{person.role}</p></div></div><span>{person.team}</span><span className={`status ${person.status.toLowerCase()}`}>{person.status}</span></div>)}
            </article>

            <article className="panel activity-panel">
              <div className="panel-heading"><div><h2>Activity</h2><p>Latest organization events</p></div><button className="text-button">View all →</button></div>
              <div className="activity-list">{activity.map(([type, text, time]) => <div className="activity" key={text}><span className="activity-icon">{type.slice(0, 2).toUpperCase()}</span><div><strong>{type}</strong><p>{text}</p></div><time>{time}</time></div>)}</div>
            </article>
          </div>

          <section className="quick-section"><div className="section-heading"><div><h2>Quick actions</h2><p>Common people operations, one click away.</p></div></div><div className="quick-grid"><button><span>＋</span><div><strong>Add employee</strong><p>Create a new employee profile</p></div></button><button><span>✓</span><div><strong>Run payroll</strong><p>Prepare this month’s pay run</p></div></button><button><span>◎</span><div><strong>Start review</strong><p>Launch a performance cycle</p></div></button><button><span>✦</span><div><strong>Ask Advance AI</strong><p>Explore workforce insights</p></div></button></div></section>
        </div>
      </section>
    </main>
  );
}

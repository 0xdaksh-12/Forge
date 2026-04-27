import { NavLink } from "react-router-dom";

const links = [
  { to: "/", icon: "dashboard", label: "Dashboard" },
  { to: "/pipelines", icon: "settings", label: "Pipelines" },
];

export function Sidebar() {
  return (
    <aside className="sidebar">
      <div className="sidebar-logo">
        <div className="sidebar-logo-icon">
          <img src="/logo.png" alt="Forge" style={{ width: "24px", height: "24px", objectFit: "contain" }} />
        </div>
        <div className="sidebar-logo-text">
          <span>Forge</span>
        </div>
      </div>

      <nav className="sidebar-nav">
        {links.map(({ to, icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === "/"}
            className={({ isActive }) => `nav-item${isActive ? " active" : ""}`}
          >
            <span className="nav-icon">
              <span className="material-icons" style={{ fontSize: "18px" }}>
                {icon}
              </span>
            </span>
            {label}
          </NavLink>
        ))}
      </nav>

      <div className="sidebar-footer">v0.1.0 — self-hosted</div>
    </aside>
  );
}

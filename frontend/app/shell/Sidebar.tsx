"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { logOff, type Me } from "@/lib/api";
import Avatar from "./Avatar";
import {
  ChevronIcon,
  CloseIcon,
  GearIcon,
  HomeIcon,
  ListPlusIcon,
  PersonPlusIcon,
  PlusIcon,
  ReportIcon,
} from "./icons";
import styles from "./shell.module.css";

export type MenuKey = "home";

// UI-008 sidebar: collapsed by default, expanded while hovered or focused
// (agreed for TBD-019). Menus whose destination is TBD-019 are shown but do
// nothing; Gold Monitoring is shown with no function (Gold is out of scope).
export default function Sidebar({
  me,
  endpointPath,
  current,
}: {
  me: Me;
  endpointPath: string;
  current: MenuKey;
}) {
  const [expanded, setExpanded] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <nav
      className={`${styles.sidebar} ${expanded ? styles.expanded : ""}`}
      aria-label="Navigasi"
      onMouseEnter={() => setExpanded(true)}
      onMouseLeave={() => setExpanded(false)}
      onFocus={() => setExpanded(true)}
      onBlur={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
          setExpanded(false);
        }
      }}
    >
      {/* "PEGA" is not shown; the logo replacement is TBD-019 (agreed: "FOS"). */}
      <div className={styles.brand}>FOS</div>

      <ul className={styles.menu}>
        {me.caseTypes.length > 0 && (
          <li>
            <MenuButton
              icon={createOpen ? <CloseIcon /> : <PlusIcon />}
              label="Create"
              expanded={expanded}
              onClick={() => setCreateOpen(!createOpen)}
              ariaExpanded={createOpen}
              trailing={<ChevronIcon open={createOpen} />}
            />
            {createOpen && expanded && (
              <ul className={styles.submenu}>
                {me.caseTypes.map((caseType) => (
                  <li key={caseType}>
                    {/* Starting a Financing case (FR-APP-001) arrives in ticket 05. */}
                    <button type="button" className={styles.submenuItem}>
                      {caseType}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </li>
        )}
        <li>
          <MenuButton
            icon={<HomeIcon />}
            label="Home"
            expanded={expanded}
            current={current === "home"}
            onClick={() => window.location.assign(endpointPath)}
          />
        </li>
        <li>
          <MenuButton icon={<ReportIcon />} label="Financing Case Summary Report" expanded={expanded} />
        </li>
        <li>
          <MenuButton icon={<ListPlusIcon />} label="Gold Monitoring" expanded={expanded} />
        </li>
        <li>
          <MenuButton icon={<GearIcon />} label="Tools" expanded={expanded} />
        </li>
        <li>
          <MenuButton icon={<PersonPlusIcon />} label="Operator Tools" expanded={expanded} />
        </li>
      </ul>

      <UserMenu me={me} endpointPath={endpointPath} expanded={expanded} />
    </nav>
  );
}

function MenuButton({
  icon,
  label,
  expanded,
  current = false,
  onClick,
  ariaExpanded,
  trailing,
}: {
  icon: ReactNode;
  label: string;
  expanded: boolean;
  current?: boolean;
  onClick?: () => void;
  ariaExpanded?: boolean;
  trailing?: ReactNode;
}) {
  return (
    <button
      type="button"
      className={`${styles.item} ${current ? styles.current : ""}`}
      aria-current={current ? "page" : undefined}
      aria-expanded={ariaExpanded}
      aria-label={expanded ? undefined : label}
      title={expanded ? undefined : label}
      onClick={onClick}
    >
      <span className={styles.icon}>{icon}</span>
      {expanded && <span className={styles.label}>{label}</span>}
      {expanded && trailing && <span className={styles.trailing}>{trailing}</span>}
    </button>
  );
}

// FR-NAV-002: clicking the avatar shows a small popup with one choice, "Log
// off", which ends the endpoint's session with no confirmation dialog.
function UserMenu({ me, endpointPath, expanded }: { me: Me; endpointPath: string; expanded: boolean }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const close = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", close);
    return () => document.removeEventListener("mousedown", close);
  }, [open]);

  return (
    <div className={styles.user} ref={ref}>
      {open && (
        <div className={styles.userPopup} role="menu">
          <button type="button" role="menuitem" className={styles.logOff} onClick={() => logOff(endpointPath, me.loginPath)}>
            Log off
          </button>
        </div>
      )}
      <button
        type="button"
        className={styles.userButton}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={expanded ? undefined : me.fullName}
        onClick={() => setOpen(!open)}
      >
        <Avatar name={me.fullName} />
        {expanded && <span className={styles.userName}>{me.fullName}</span>}
      </button>
    </div>
  );
}

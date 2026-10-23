"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { apiFetch, assignmentPath, logOff, type CaseType, type Me } from "@/lib/api";
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

// The current menu, if the page belongs to one (Case View belongs to none).
export type MenuKey = "home" | null;

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

  return (
    <nav
      className={`${styles.sidebar} ${expanded ? styles.expanded : ""}`}
      aria-label="Navigation"
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
            {/* Remounted per expansion, so Create starts closed each time. */}
            <CreateMenu key={String(expanded)} caseTypes={me.caseTypes} expanded={expanded} endpointPath={endpointPath} />
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
}: {
  icon: ReactNode;
  label: string;
  expanded: boolean;
  current?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      className={`${styles.item} ${current ? styles.current : ""}`}
      aria-current={current ? "page" : undefined}
      aria-label={expanded ? undefined : label}
      title={expanded ? undefined : label}
      onClick={onClick}
    >
      <span className={styles.icon}>{icon}</span>
      {expanded && <span className={styles.label}>{label}</span>}
    </button>
  );
}

// FR-NAV-003: closed shows + and a right arrow; open shows X, a down arrow
// and the parent case types underneath (UI-008, Gambar 7 and 8).
function CreateMenu({
  caseTypes,
  expanded,
  endpointPath,
}: {
  caseTypes: CaseType[];
  expanded: boolean;
  endpointPath: string;
}) {
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);

  // FR-APP-001: choosing a case type creates the case at once (TBD-023a,
  // agreed) and opens its first assignment in Case View.
  async function create(caseType: CaseType) {
    if (creating) {
      return;
    }
    setCreating(true);
    const res = await apiFetch(`${endpointPath}/api/cases`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ caseType: caseType.code }),
    }).catch(() => null);
    if (res?.status === 201) {
      const body = (await res.json()) as { caseId: string; assignmentId: number };
      window.location.assign(assignmentPath(endpointPath, body.caseId, body.assignmentId));
      return;
    }
    setCreating(false);
  }
  return (
    <>
      <button
        type="button"
        className={styles.item}
        aria-expanded={open}
        aria-label={expanded ? undefined : "Create"}
        title={expanded ? undefined : "Create"}
        onClick={() => setOpen(!open)}
      >
        <span className={styles.icon}>{open ? <CloseIcon /> : <PlusIcon />}</span>
        {expanded && <span className={styles.label}>Create</span>}
        {expanded && (
          <span className={styles.trailing}>
            <ChevronIcon open={open} />
          </span>
        )}
      </button>
      {open && expanded && (
        <ul className={styles.submenu}>
          {caseTypes.map((caseType) => (
            <li key={caseType.code}>
              <button type="button" className={styles.submenuItem} disabled={creating} onClick={() => create(caseType)}>
                {caseType.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </>
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

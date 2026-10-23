// Menu icons approximating UI-008 (final icons are TBD-019).

const common = { width: 18, height: 18, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: 2 };

export function PlusIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M12 5v14M5 12h14" strokeLinecap="round" />
    </svg>
  );
}

export function CloseIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M6 6l12 12M18 6L6 18" strokeLinecap="round" />
    </svg>
  );
}

export function HomeIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6h-6v6H4a1 1 0 0 1-1-1z" strokeLinejoin="round" />
    </svg>
  );
}

export function ReportIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M6 3h9l4 4v14H6z" strokeLinejoin="round" />
      <path d="M9 12h7M9 16h7M9 8h4" strokeLinecap="round" />
    </svg>
  );
}

export function ListPlusIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M4 6h10M4 12h10M4 18h6M18 14v6M15 17h6" strokeLinecap="round" />
    </svg>
  );
}

export function GearIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <circle cx="12" cy="12" r="3" />
      <path
        d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function PersonPlusIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <circle cx="10" cy="8" r="4" />
      <path d="M3 21c0-4 3-6 7-6s7 2 7 6M19 8v6M16 11h6" strokeLinecap="round" />
    </svg>
  );
}

export function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg {...common} width={14} height={14} aria-hidden="true">
      <path d={open ? "M6 9l6 6 6-6" : "M9 6l6 6-6 6"} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

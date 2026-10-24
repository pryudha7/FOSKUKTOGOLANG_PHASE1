"use client";

import { useState } from "react";
import NoAccessPopup from "./NoAccessPopup";

// ERR-039: an initial dropdown could not be loaded (BR-UI-004); shown once
// as the ERR-035 popup with OK.
export const dropdownsFailedMessage =
  "Gagal mendapatkan data dropdown, harap refresh halaman. Jika kesalahan berlanjut, hubungi helpdesk";

export default function DropdownsFailedPopup({ failed }: { failed: string[] }) {
  const [open, setOpen] = useState(failed.length > 0);
  return open ? <NoAccessPopup message={dropdownsFailedMessage} onClose={() => setOpen(false)} /> : null;
}

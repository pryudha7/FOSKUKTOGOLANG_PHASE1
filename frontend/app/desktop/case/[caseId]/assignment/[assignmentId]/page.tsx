import { endpoints } from "@/lib/env";
import DesktopShell from "@/app/shell/DesktopShell";
import CaseView from "@/app/case/CaseView";

// UI-013 Case View in mode Assignment (FR-CASE-008), served at
// <ENDPOINT_PATH_DESKTOP>/case/<caseId>/assignment/<assignmentId>.
export const dynamic = "force-dynamic";

export default async function AssignmentPage({
  params,
}: {
  params: Promise<{ caseId: string; assignmentId: string }>;
}) {
  const { caseId, assignmentId } = await params;
  const { desktop } = endpoints();
  return (
    <DesktopShell endpointPath={desktop.path} loginPath={desktop.defaultLoginPath} current={null} layout="case">
      <CaseView endpointPath={desktop.path} caseId={decodeURIComponent(caseId)} assignmentId={Number(assignmentId)} />
    </DesktopShell>
  );
}

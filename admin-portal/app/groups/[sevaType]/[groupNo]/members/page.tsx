import { GroupMembersClient } from "@/components/group-members-client";

export default function GroupMembersPage({
  params
}: {
  params: { sevaType: string; groupNo: string };
}) {
  return <GroupMembersClient sevaType={params.sevaType} groupNo={Number(params.groupNo)} />;
}

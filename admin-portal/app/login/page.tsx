import { LoginForm } from "@/components/auth-client";

export default function LoginPage() {
  return (
    <div className="max-w-md mx-auto">
      <div className="rounded-2xl bg-panel/60 border border-black/10 shadow-soft p-5">
        <div className="text-sm text-muted">Admin</div>
        <h1 className="text-xl font-semibold tracking-tight">Sign in</h1>
        <div className="mt-4">
          <LoginForm />
        </div>
      </div>
    </div>
  );
}

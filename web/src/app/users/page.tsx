"use client";

import { useEffect, useState } from "react";
import { LoaderCircle, RefreshCw, ShieldCheck, UserPlus, Users } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { createUser, fetchUsers, updateUser, type User, type UserRole } from "@/lib/api";

export default function UsersPage() {
  const [items, setItems] = useState<User[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isCreating, setIsCreating] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<UserRole>("user");
  const [passwordDrafts, setPasswordDrafts] = useState<Record<string, string>>({});

  const loadUsers = async () => {
    setIsLoading(true);
    try {
      const payload = await fetchUsers();
      setItems(payload.items);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "读取用户失败");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadUsers();
  }, []);

  const handleCreate = async () => {
    if (!username.trim() || !password) {
      toast.error("请输入用户名和密码");
      return;
    }
    setIsCreating(true);
    try {
      await createUser({ username, password, role });
      setUsername("");
      setPassword("");
      setRole("user");
      await loadUsers();
      toast.success("用户已创建");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "创建用户失败");
    } finally {
      setIsCreating(false);
    }
  };

  const patchUser = async (id: string, payload: Parameters<typeof updateUser>[1]) => {
    try {
      await updateUser(id, payload);
      await loadUsers();
      toast.success("用户已更新");
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "更新用户失败");
    }
  };

  return (
    <section className="h-full overflow-y-auto">
      <div className="mx-auto max-w-[1280px] px-1 py-1">
        <div className="rounded-[28px] border border-stone-200 bg-[#fcfcfb] px-4 py-5 shadow-[0_14px_40px_rgba(15,23,42,0.05)] sm:px-6">
          <div className="flex flex-col gap-6">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="flex items-center gap-4">
                <span className="inline-flex size-12 items-center justify-center rounded-[18px] bg-stone-950 text-white">
                  <Users className="size-5" />
                </span>
                <div>
                  <h1 className="text-2xl font-semibold tracking-tight text-stone-950">用户管理</h1>
                </div>
              </div>
              <Button
                type="button"
                variant="outline"
                className="h-10 rounded-full border-stone-200 bg-white"
                onClick={() => void loadUsers()}
                disabled={isLoading}
              >
                <RefreshCw className={isLoading ? "size-4 animate-spin" : "size-4"} />
                刷新
              </Button>
            </div>

            <div className="grid gap-3 rounded-2xl border border-stone-200 bg-white p-4 lg:grid-cols-[1fr_1fr_160px_auto]">
              <Input
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder="用户名"
                autoComplete="off"
                className="h-11 rounded-xl border-stone-200 bg-stone-50"
              />
              <Input
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="初始密码"
                type="password"
                autoComplete="new-password"
                className="h-11 rounded-xl border-stone-200 bg-stone-50"
              />
              <select
                value={role}
                onChange={(event) => setRole(event.target.value as UserRole)}
                className="h-11 rounded-xl border border-stone-200 bg-stone-50 px-3 text-sm text-stone-700 outline-none"
              >
                <option value="user">普通用户</option>
                <option value="admin">管理员</option>
              </select>
              <Button
                type="button"
                className="h-11 rounded-xl bg-stone-950 text-white"
                onClick={() => void handleCreate()}
                disabled={isCreating}
              >
                {isCreating ? <LoaderCircle className="size-4 animate-spin" /> : <UserPlus className="size-4" />}
                创建
              </Button>
            </div>

            <div className="overflow-hidden rounded-2xl border border-stone-200 bg-white">
              <div className="hidden grid-cols-[1fr_120px_120px_1.4fr] border-b border-stone-100 bg-stone-50 px-4 py-3 text-xs font-medium uppercase tracking-[0.16em] text-stone-400 lg:grid">
                <div>用户</div>
                <div>角色</div>
                <div>状态</div>
                <div>操作</div>
              </div>
              <div className="divide-y divide-stone-100">
                {items.map((item) => (
                  <div key={item.id} className="grid gap-3 px-4 py-4 lg:grid-cols-[1fr_120px_120px_1.4fr] lg:items-center">
                    <div className="min-w-0">
                      <div className="truncate font-medium text-stone-900">{item.username}</div>
                      <div className="mt-1 truncate text-xs text-stone-400">{item.id}</div>
                    </div>
                    <Badge variant={item.role === "admin" ? "info" : "secondary"} className="w-fit rounded-md px-2 py-1">
                      {item.role === "admin" ? "管理员" : "普通用户"}
                    </Badge>
                    <Badge variant={item.disabled ? "danger" : "success"} className="w-fit rounded-md px-2 py-1">
                      {item.disabled ? "已禁用" : "启用中"}
                    </Badge>
                    <div className="grid gap-2 sm:grid-cols-[130px_1fr_auto_auto]">
                      <select
                        value={item.role}
                        onChange={(event) => void patchUser(item.id, { role: event.target.value as UserRole })}
                        className="h-10 rounded-xl border border-stone-200 bg-stone-50 px-3 text-sm text-stone-700 outline-none"
                      >
                        <option value="user">普通用户</option>
                        <option value="admin">管理员</option>
                      </select>
                      <Input
                        value={passwordDrafts[item.id] ?? ""}
                        onChange={(event) =>
                          setPasswordDrafts((current) => ({ ...current, [item.id]: event.target.value }))
                        }
                        placeholder="新密码"
                        type="password"
                        autoComplete="new-password"
                        className="h-10 rounded-xl border-stone-200 bg-stone-50"
                      />
                      <Button
                        type="button"
                        variant="outline"
                        className="h-10 rounded-xl border-stone-200 bg-white"
                        onClick={() => {
                          const nextPassword = passwordDrafts[item.id]?.trim();
                          if (!nextPassword) {
                            toast.error("请输入新密码");
                            return;
                          }
                          void patchUser(item.id, { password: nextPassword });
                          setPasswordDrafts((current) => ({ ...current, [item.id]: "" }));
                        }}
                      >
                        <ShieldCheck className="size-4" />
                        重置
                      </Button>
                      <Button
                        type="button"
                        variant={item.disabled ? "default" : "outline"}
                        className="h-10 rounded-xl"
                        onClick={() => void patchUser(item.id, { disabled: !item.disabled })}
                      >
                        {item.disabled ? "启用" : "禁用"}
                      </Button>
                    </div>
                  </div>
                ))}
                {!isLoading && items.length === 0 ? (
                  <div className="px-4 py-12 text-center text-sm text-stone-500">暂无用户</div>
                ) : null}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

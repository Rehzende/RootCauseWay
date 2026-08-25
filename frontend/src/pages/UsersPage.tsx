import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listUsers, createUser, updateUser, deleteUser, listRoles, assignRole, unassignRole } from '@/services/api';
import { PermissionGate } from '@/components/PermissionGate';
import { useToast } from '@/components/Toast';
import { X, UserPlus, Trash2 } from 'lucide-react';
import type { UserWithRoles, RoleWithPermissions } from '@/types/api';

function UserAvatar({ name, avatar_url, size = 'sm' }: { name: string; avatar_url?: string; size?: 'sm' | 'md' }) {
  const dim = size === 'sm' ? 'h-8 w-8 text-sm' : 'h-10 w-10 text-base';
  if (avatar_url) {
    return <img src={avatar_url} alt={name} className={`${dim} rounded-full object-cover`} />;
  }
  return (
    <div className={`${dim} flex items-center justify-center rounded-full bg-blue-500 font-bold text-white`}>
      {name?.charAt(0)?.toUpperCase() ?? 'U'}
    </div>
  );
}

function CreateUserModal({ roles, onClose }: { roles: RoleWithPermissions[]; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [selectedRoles, setSelectedRoles] = useState<string[]>([]);

  const { addToast } = useToast();
  const mutation = useMutation({
    mutationFn: () => createUser({ name, email, password, role_ids: selectedRoles }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['users'] }); addToast({ type: 'success', title: 'User created successfully' }); onClose(); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create user', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">Create User</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Email</label>
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Password</label>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Roles</label>
            <div className="mt-2 space-y-2">
              {roles.map((role) => (
                <label key={role.id} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={selectedRoles.includes(role.id)}
                    onChange={(e) => setSelectedRoles(e.target.checked ? [...selectedRoles, role.id] : selectedRoles.filter((r) => r !== role.id))}
                    className="rounded border-gray-300"
                  />
                  {role.name}
                </label>
              ))}
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <button type="button" onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Cancel</button>
            <button type="submit" disabled={mutation.isPending} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {mutation.isPending ? 'Creating...' : 'Create'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

function UserDetailModal({ user, roles, onClose }: { user: UserWithRoles; roles: RoleWithPermissions[]; onClose: () => void }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);

  const { addToast } = useToast();
  const updateMutation = useMutation({
    mutationFn: () => updateUser(user.id, { name, email }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['users'] }); addToast({ type: 'success', title: 'User updated' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update user', message: err?.response?.data?.error || err.message }); },
  });

  const toggleActiveMutation = useMutation({
    mutationFn: () => updateUser(user.id, { is_active: !user.is_active }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['users'] }); addToast({ type: 'success', title: user.is_active ? 'User deactivated' : 'User activated' }); onClose(); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update user', message: err?.response?.data?.error || err.message }); },
  });

  const assignMutation = useMutation({
    mutationFn: (roleId: string) => assignRole(user.id, roleId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['users'] }); addToast({ type: 'success', title: 'Role assigned' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to assign role', message: err?.response?.data?.error || err.message }); },
  });

  const unassignMutation = useMutation({
    mutationFn: (roleId: string) => unassignRole(user.id, roleId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['users'] }); addToast({ type: 'success', title: 'Role removed' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to remove role', message: err?.response?.data?.error || err.message }); },
  });

  const userRoleIds = user.roles?.map((r) => r.id) ?? [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-lg rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">User Details</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <div className="space-y-4">
          <div className="flex items-center gap-3">
            <UserAvatar name={user.name} avatar_url={user.avatar_url} size="md" />
            <div>
              <p className="font-medium text-gray-900">{user.name}</p>
              <p className="text-sm text-gray-500">{user.email}</p>
              {user.sso_provider && (
                <span className="mt-1 inline-block rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700">SSO: {user.sso_provider}</span>
              )}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm font-medium text-gray-700">Name</label>
              <input value={name} onChange={(e) => setName(e.target.value)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700">Email</label>
              <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
            </div>
          </div>

          <PermissionGate resource="users" action="write">
            <button onClick={() => updateMutation.mutate()} disabled={updateMutation.isPending} className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </button>
          </PermissionGate>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Roles</label>
            <div className="space-y-1">
              {roles.map((role) => (
                <label key={role.id} className="flex items-center gap-2 text-sm">
                  <PermissionGate resource="roles" action="write" fallback={
                    <input type="checkbox" checked={userRoleIds.includes(role.id)} disabled className="rounded border-gray-300" />
                  }>
                    <input
                      type="checkbox"
                      checked={userRoleIds.includes(role.id)}
                      onChange={(e) => e.target.checked ? assignMutation.mutate(role.id) : unassignMutation.mutate(role.id)}
                      className="rounded border-gray-300"
                    />
                  </PermissionGate>
                  {role.name}
                  {role.is_system && <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500">system</span>}
                </label>
              ))}
            </div>
          </div>

          {user.last_login_at && (
            <p className="text-xs text-gray-500">Last login: {new Date(user.last_login_at).toLocaleString()}</p>
          )}

          <div className="flex items-center justify-between border-t border-gray-200 pt-4">
            <PermissionGate resource="users" action="write">
              <button
                onClick={() => toggleActiveMutation.mutate()}
                className={`rounded-md px-3 py-1.5 text-sm font-medium ${user.is_active ? 'bg-red-50 text-red-700 hover:bg-red-100' : 'bg-green-50 text-green-700 hover:bg-green-100'}`}
              >
                {user.is_active ? 'Deactivate' : 'Activate'}
              </button>
            </PermissionGate>
            <button onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Close</button>
          </div>
        </div>
      </div>
    </div>
  );
}

export function UsersPage() {
  const [showCreate, setShowCreate] = useState(false);
  const [selectedUser, setSelectedUser] = useState<UserWithRoles | null>(null);

  const { data: usersData, isLoading } = useQuery({ queryKey: ['users'], queryFn: () => listUsers({ per_page: 100 }) });
  const { data: rolesData } = useQuery({ queryKey: ['roles'], queryFn: () => listRoles({ per_page: 100 }) });
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['users'] }); addToast({ type: 'success', title: 'User deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete user', message: err?.response?.data?.error || err.message }); },
  });

  const users = usersData?.data ?? [];
  const roles = rolesData?.data ?? [];

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Users</h1>
          <p className="mt-1 text-sm text-gray-500">Manage user accounts and role assignments</p>
        </div>
        <PermissionGate resource="users" action="write">
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
            <UserPlus className="h-4 w-4" /> Add User
          </button>
        </PermissionGate>
      </div>

      {isLoading ? (
        <div className="mt-8 text-sm text-gray-500">Loading users...</div>
      ) : (
        <div className="mt-6 overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">User</th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Email</th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Roles</th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">SSO</th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Last Login</th>
                <th className="px-6 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Status</th>
                <th className="px-6 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {users.map((u) => (
                <tr key={u.id} className="cursor-pointer hover:bg-gray-50" onClick={() => setSelectedUser(u)}>
                  <td className="whitespace-nowrap px-6 py-4">
                    <div className="flex items-center gap-3">
                      <UserAvatar name={u.name} avatar_url={u.avatar_url} />
                      <span className="text-sm font-medium text-gray-900">{u.name}</span>
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">{u.email}</td>
                  <td className="px-6 py-4">
                    <div className="flex flex-wrap gap-1">
                      {u.roles?.map((r) => (
                        <span key={r.id} className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-700">{r.name}</span>
                      ))}
                    </div>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm">
                    {u.sso_provider ? (
                      <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-700">{u.sso_provider}</span>
                    ) : (
                      <span className="text-gray-400">-</span>
                    )}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-sm text-gray-500">
                    {u.last_login_at ? new Date(u.last_login_at).toLocaleDateString() : '-'}
                  </td>
                  <td className="whitespace-nowrap px-6 py-4">
                    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${u.is_active ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
                      {u.is_active ? 'Active' : 'Inactive'}
                    </span>
                  </td>
                  <td className="whitespace-nowrap px-6 py-4 text-right" onClick={(e) => e.stopPropagation()}>
                    <PermissionGate resource="users" action="write">
                      <button onClick={() => { if (confirm('Delete this user?')) deleteMutation.mutate(u.id); }} className="text-gray-400 hover:text-red-600">
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </PermissionGate>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showCreate && <CreateUserModal roles={roles} onClose={() => setShowCreate(false)} />}
      {selectedUser && <UserDetailModal user={selectedUser} roles={roles} onClose={() => setSelectedUser(null)} />}
    </div>
  );
}

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { listRoles, createRole, updateRole, deleteRole, listPermissions, grantPermission, revokePermission } from '@/services/api';
import { PermissionGate } from '@/components/PermissionGate';
import { X, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { useToast } from '@/components/Toast';
import type { RoleWithPermissions, Permission } from '@/types/api';

function CreateRoleModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [description, setDescription] = useState('');

  const mutation = useMutation({
    mutationFn: () => createRole({ name, slug, description }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['roles'] }); onClose(); addToast({ type: 'success', title: 'Role created successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to create role', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-md rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900">Create Role</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>
        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(); }} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700">Name</label>
            <input value={name} onChange={(e) => setName(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Slug</label>
            <input value={slug} onChange={(e) => setSlug(e.target.value)} required className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700">Description</label>
            <textarea value={description} onChange={(e) => setDescription(e.target.value)} rows={3} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500" />
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

function PermissionMatrix({ role, allPermissions }: { role: RoleWithPermissions; allPermissions: Permission[] }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();

  const grantMut = useMutation({
    mutationFn: (permId: string) => grantPermission(role.id, permId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['roles'] }); addToast({ type: 'success', title: 'Permission granted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to grant permission', message: err?.response?.data?.error || err.message }); },
  });
  const revokeMut = useMutation({
    mutationFn: (permId: string) => revokePermission(role.id, permId),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['roles'] }); addToast({ type: 'success', title: 'Permission revoked' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to revoke permission', message: err?.response?.data?.error || err.message }); },
  });

  // Group permissions by resource
  const resources = [...new Set(allPermissions.map((p) => p.resource))].sort();
  const actions = [...new Set(allPermissions.map((p) => p.action))].sort();
  const rolePermIds = new Set(role.permissions?.map((p) => p.id) ?? []);

  const findPerm = (resource: string, action: string) =>
    allPermissions.find((p) => p.resource === resource && p.action === action);

  return (
    <div className="overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead>
          <tr>
            <th className="px-3 py-2 text-left text-xs font-medium uppercase text-gray-500">Resource</th>
            {actions.map((a) => (
              <th key={a} className="px-3 py-2 text-center text-xs font-medium uppercase text-gray-500">{a}</th>
            ))}
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {resources.map((resource) => (
            <tr key={resource}>
              <td className="px-3 py-2 font-medium text-gray-700">{resource}</td>
              {actions.map((action) => {
                const perm = findPerm(resource, action);
                if (!perm) return <td key={action} className="px-3 py-2 text-center"><span className="text-gray-300">-</span></td>;
                const granted = rolePermIds.has(perm.id);
                return (
                  <td key={action} className="px-3 py-2 text-center">
                    <input
                      type="checkbox"
                      checked={granted}
                      disabled={role.is_system}
                      onChange={() => granted ? revokeMut.mutate(perm.id) : grantMut.mutate(perm.id)}
                      className={`rounded border-gray-300 ${granted ? 'text-green-600' : ''} ${role.is_system ? 'cursor-not-allowed opacity-50' : ''}`}
                    />
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function RoleDetailModal({ role, allPermissions, onClose }: { role: RoleWithPermissions; allPermissions: Permission[]; onClose: () => void }) {
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const [name, setName] = useState(role.name);
  const [description, setDescription] = useState(role.description ?? '');

  const updateMut = useMutation({
    mutationFn: () => updateRole(role.id, { name, description }),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['roles'] }); addToast({ type: 'success', title: 'Role updated successfully' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to update role', message: err?.response?.data?.error || err.message }); },
  });

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="w-full max-w-3xl max-h-[90vh] overflow-y-auto rounded-lg bg-white p-6 shadow-xl">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <h3 className="text-lg font-semibold text-gray-900">{role.name}</h3>
            {role.is_system && <span className="rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">system</span>}
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600"><X className="h-5 w-5" /></button>
        </div>

        {!role.is_system && (
          <div className="mb-6 space-y-3">
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700">Name</label>
                <input value={name} onChange={(e) => setName(e.target.value)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700">Description</label>
                <input value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1 block w-full rounded-md border border-gray-300 px-3 py-2 text-sm" />
              </div>
            </div>
            <PermissionGate resource="roles" action="write">
              <button onClick={() => updateMut.mutate()} disabled={updateMut.isPending} className="rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-blue-700 disabled:opacity-50">
                {updateMut.isPending ? 'Saving...' : 'Save'}
              </button>
            </PermissionGate>
          </div>
        )}

        <h4 className="mb-2 text-sm font-medium text-gray-900">Permission Matrix</h4>
        <PermissionMatrix role={role} allPermissions={allPermissions} />

        <div className="mt-4 flex justify-end">
          <button onClick={onClose} className="rounded-md border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50">Close</button>
        </div>
      </div>
    </div>
  );
}

export function RolesPage() {
  const [showCreate, setShowCreate] = useState(false);
  const [selectedRole, setSelectedRole] = useState<RoleWithPermissions | null>(null);

  const { data: rolesData, isLoading } = useQuery({ queryKey: ['roles'], queryFn: () => listRoles({ per_page: 100 }) });
  const { data: permissionsData } = useQuery({ queryKey: ['permissions'], queryFn: listPermissions });
  const queryClient = useQueryClient();
  const { addToast } = useToast();
  const deleteMut = useMutation({
    mutationFn: deleteRole,
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['roles'] }); addToast({ type: 'success', title: 'Role deleted' }); },
    onError: (err: any) => { addToast({ type: 'error', title: 'Failed to delete role', message: err?.response?.data?.error || err.message }); },
  });

  const roles = rolesData?.data ?? [];
  const allPermissions = permissionsData ?? [];

  return (
    <div className="p-8">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Roles</h1>
          <p className="mt-1 text-sm text-gray-500">Manage roles and their permissions</p>
        </div>
        <PermissionGate resource="roles" action="write">
          <button onClick={() => setShowCreate(true)} className="flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-medium text-white hover:bg-blue-700">
            <Plus className="h-4 w-4" /> Create Role
          </button>
        </PermissionGate>
      </div>

      {isLoading ? (
        <div className="mt-8 text-sm text-gray-500">Loading roles...</div>
      ) : (
        <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {roles.map((role) => (
            <div key={role.id} className="cursor-pointer rounded-lg border border-gray-200 bg-white p-5 shadow-sm transition-shadow hover:shadow-md" onClick={() => setSelectedRole(role)}>
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-2">
                  <ShieldCheck className="h-5 w-5 text-blue-500" />
                  <h3 className="font-medium text-gray-900">{role.name}</h3>
                </div>
                <div className="flex items-center gap-1">
                  {role.is_system && <span className="rounded bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-500">system</span>}
                  {!role.is_system && (
                    <PermissionGate resource="roles" action="delete">
                      <button onClick={(e) => { e.stopPropagation(); if (confirm('Delete this role?')) deleteMut.mutate(role.id); }} className="text-gray-400 hover:text-red-600">
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </PermissionGate>
                  )}
                </div>
              </div>
              {role.description && <p className="mt-2 text-sm text-gray-500">{role.description}</p>}
              <div className="mt-3 flex items-center gap-3 text-xs text-gray-400">
                <span>{role.permissions?.length ?? 0} permissions</span>
              </div>
            </div>
          ))}
        </div>
      )}

      {showCreate && <CreateRoleModal onClose={() => setShowCreate(false)} />}
      {selectedRole && <RoleDetailModal role={selectedRole} allPermissions={allPermissions} onClose={() => setSelectedRole(null)} />}
    </div>
  );
}

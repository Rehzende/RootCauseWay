import { describe, it, expect, vi, beforeEach } from 'vitest';
import api, {
  login,
  listSoftware,
  createSoftware,
  getSoftware,
  updateSoftware,
  deleteSoftware,
  listAgents,
  createAgent,
  getAgent,
  updateAgent,
  deleteAgent,
  listWebhooks,
  createWebhook,
  deleteWebhook,
  listIncidents,
  getIncident,
  updateIncident,
  addIncidentEvent,
  addIncidentEvidence,
  getAnalyticsIncidentTrends,
  listNotificationChannels,
  createNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  listEscalationPolicies,
  createEscalationPolicy,
  listMarketplaceAgents,
  getMarketplaceAgent,
  installAgent,
  listAccessPolicies,
  createAccessPolicy,
  listSSOProviders,
  createSSOProvider,
  updateSSOProvider,
  deleteSSOProvider,
  listCorrelationRules,
  listAgentSkills,
  linkSkillToAgent,
  unlinkSkillFromAgent,
  getRunbookExecution,
  updateSnapshotConfig,
  deleteSnapshotConfig,
  searchKnowledgeBase,
  listRunbooks,
} from '@/services/api';

vi.spyOn(api, 'get').mockResolvedValue({ data: {} });
vi.spyOn(api, 'post').mockResolvedValue({ data: {} });
vi.spyOn(api, 'put').mockResolvedValue({ data: {} });
vi.spyOn(api, 'patch').mockResolvedValue({ data: {} });
vi.spyOn(api, 'delete').mockResolvedValue({ data: {} });

beforeEach(() => {
  vi.clearAllMocks();
});

describe('Auth API', () => {
  it('login calls POST /auth/login', async () => {
    await login({ email: 'a@b.com', password: 'password1' });
    expect(api.post).toHaveBeenCalledWith('/auth/login', { email: 'a@b.com', password: 'password1' });
  });
});

describe('Software API', () => {
  it('listSoftware calls GET /software with pagination', async () => {
    await listSoftware(2, 10);
    expect(api.get).toHaveBeenCalledWith('/software', { params: { page: 2, per_page: 10 } });
  });

  it('createSoftware calls POST /software', async () => {
    const payload = { name: 'Test', slug: 'test' };
    await createSoftware(payload);
    expect(api.post).toHaveBeenCalledWith('/software', payload);
  });

  it('getSoftware calls GET /software/:id', async () => {
    await getSoftware('abc');
    expect(api.get).toHaveBeenCalledWith('/software/abc');
  });

  it('updateSoftware calls PUT /software/:id', async () => {
    const payload = { name: 'Updated', slug: 'updated' };
    await updateSoftware('abc', payload);
    expect(api.put).toHaveBeenCalledWith('/software/abc', payload);
  });

  it('deleteSoftware calls DELETE /software/:id', async () => {
    await deleteSoftware('abc');
    expect(api.delete).toHaveBeenCalledWith('/software/abc');
  });
});

describe('Agents API', () => {
  it('listAgents calls GET /agents', async () => {
    await listAgents({ page: 1 });
    expect(api.get).toHaveBeenCalledWith('/agents', { params: { page: 1 } });
  });

  it('createAgent calls POST /agents', async () => {
    const payload = { name: 'Bot', type: 'triage' as const };
    await createAgent(payload);
    expect(api.post).toHaveBeenCalledWith('/agents', payload);
  });

  it('getAgent calls GET /agents/:id', async () => {
    await getAgent('x');
    expect(api.get).toHaveBeenCalledWith('/agents/x');
  });

  it('updateAgent calls PUT /agents/:id', async () => {
    const payload = { name: 'Bot2', type: 'debug' as const };
    await updateAgent('x', payload);
    expect(api.put).toHaveBeenCalledWith('/agents/x', payload);
  });

  it('deleteAgent calls DELETE /agents/:id', async () => {
    await deleteAgent('x');
    expect(api.delete).toHaveBeenCalledWith('/agents/x');
  });
});

describe('Webhooks API', () => {
  it('listWebhooks calls GET /webhooks', async () => {
    await listWebhooks();
    expect(api.get).toHaveBeenCalledWith('/webhooks');
  });

  it('createWebhook calls POST /webhooks', async () => {
    const payload = { name: 'WH', source: 'datadog' as const, software_id: 'sid' };
    await createWebhook(payload);
    expect(api.post).toHaveBeenCalledWith('/webhooks', payload);
  });

  it('deleteWebhook calls DELETE /webhooks/:id', async () => {
    await deleteWebhook('wid');
    expect(api.delete).toHaveBeenCalledWith('/webhooks/wid');
  });
});

describe('Incidents API', () => {
  it('listIncidents calls GET /incidents with filters', async () => {
    await listIncidents({ status: 'open', page: 1 });
    expect(api.get).toHaveBeenCalledWith('/incidents', { params: { status: 'open', page: 1 } });
  });

  it('getIncident calls GET /incidents/:id', async () => {
    await getIncident('inc1');
    expect(api.get).toHaveBeenCalledWith('/incidents/inc1');
  });

  it('updateIncident calls PATCH /incidents/:id', async () => {
    await updateIncident('inc1', { status: 'resolved' });
    expect(api.patch).toHaveBeenCalledWith('/incidents/inc1', { status: 'resolved' });
  });

  it('addIncidentEvent calls POST /incidents/:id/events', async () => {
    const payload = { type: 'comment' as const, data: { message: 'hi' } };
    await addIncidentEvent('inc1', payload);
    expect(api.post).toHaveBeenCalledWith('/incidents/inc1/events', payload);
  });

  it('addIncidentEvidence calls POST /incidents/:id/evidence', async () => {
    const payload = { type: 'log' as const, title: 'Log', content: { text: 'err' } };
    await addIncidentEvidence('inc1', payload);
    expect(api.post).toHaveBeenCalledWith('/incidents/inc1/evidence', payload);
  });
});

// Regression coverage for a whole class of bug a platform audit found: these
// functions called a plausible-looking but nonexistent nested path (e.g.
// "/notifications/channels") while the backend registered a different,
// flat one (e.g. "/notification-channels") -- silently 404ing in
// production with no test anywhere pinning the literal path string. Every
// endpoint below was actually broken end-to-end until this fix.
describe('Analytics API path regressions', () => {
  it('getAnalyticsIncidentTrends calls GET /analytics/trends (not /analytics/incident-trends)', async () => {
    await getAnalyticsIncidentTrends();
    expect(api.get).toHaveBeenCalledWith('/analytics/trends', { params: undefined });
  });
});

describe('Notification Channels API path regressions', () => {
  it('listNotificationChannels calls GET /notification-channels', async () => {
    await listNotificationChannels();
    expect(api.get).toHaveBeenCalledWith('/notification-channels');
  });

  it('createNotificationChannel calls POST /notification-channels', async () => {
    const payload = { name: 'Slack', type: 'slack' };
    await createNotificationChannel(payload);
    expect(api.post).toHaveBeenCalledWith('/notification-channels', payload);
  });

  it('updateNotificationChannel calls PUT /notification-channels/:id', async () => {
    await updateNotificationChannel('ch1', { name: 'Updated' });
    expect(api.put).toHaveBeenCalledWith('/notification-channels/ch1', { name: 'Updated' });
  });

  it('deleteNotificationChannel calls DELETE /notification-channels/:id', async () => {
    await deleteNotificationChannel('ch1');
    expect(api.delete).toHaveBeenCalledWith('/notification-channels/ch1');
  });
});

describe('Escalation Policies API path regressions', () => {
  it('listEscalationPolicies calls GET /escalation-policies', async () => {
    await listEscalationPolicies();
    expect(api.get).toHaveBeenCalledWith('/escalation-policies');
  });

  it('createEscalationPolicy calls POST /escalation-policies', async () => {
    const payload = { name: 'Primary' };
    await createEscalationPolicy(payload);
    expect(api.post).toHaveBeenCalledWith('/escalation-policies', payload);
  });
});

describe('Marketplace API path regressions', () => {
  it('listMarketplaceAgents calls GET /marketplace (not /marketplace/agents)', async () => {
    await listMarketplaceAgents();
    expect(api.get).toHaveBeenCalledWith('/marketplace', { params: undefined });
  });

  it('getMarketplaceAgent calls GET /marketplace/:slug', async () => {
    await getMarketplaceAgent('rca-pro');
    expect(api.get).toHaveBeenCalledWith('/marketplace/rca-pro');
  });

  it('installAgent calls POST /marketplace/:slug/install', async () => {
    await installAgent('rca-pro');
    expect(api.post).toHaveBeenCalledWith('/marketplace/rca-pro/install', { config: undefined });
  });
});

describe('Access Policies API path regressions', () => {
  it('listAccessPolicies calls GET /access-policies (not /credentials/policies)', async () => {
    await listAccessPolicies();
    expect(api.get).toHaveBeenCalledWith('/access-policies');
  });

  it('createAccessPolicy calls POST /access-policies', async () => {
    const payload = { name: 'default', software_id: 'sw1', resource_type: 'database', allowed_actions: ['read'] };
    await createAccessPolicy(payload as any);
    expect(api.post).toHaveBeenCalledWith('/access-policies', payload);
  });
});

describe('SSO Providers API path regressions', () => {
  it('listSSOProviders calls GET /sso-providers (not /auth/sso/providers)', async () => {
    await listSSOProviders();
    expect(api.get).toHaveBeenCalledWith('/sso-providers');
  });

  it('createSSOProvider calls POST /sso-providers', async () => {
    const payload = { name: 'Google', provider_type: 'google' as const, client_id: 'cid', client_secret: 'sec' };
    await createSSOProvider(payload);
    expect(api.post).toHaveBeenCalledWith('/sso-providers', payload);
  });

  it('updateSSOProvider calls PATCH /sso-providers/:id', async () => {
    await updateSSOProvider('p1', { name: 'Updated' });
    expect(api.patch).toHaveBeenCalledWith('/sso-providers/p1', { name: 'Updated' });
  });

  it('deleteSSOProvider calls DELETE /sso-providers/:id', async () => {
    await deleteSSOProvider('p1');
    expect(api.delete).toHaveBeenCalledWith('/sso-providers/p1');
  });
});

describe('Correlation Rules API path regressions', () => {
  it('listCorrelationRules calls GET /correlation-rules (not /correlation/rules)', async () => {
    await listCorrelationRules();
    expect(api.get).toHaveBeenCalledWith('/correlation-rules');
  });
});

describe('Agent Skills API path regressions', () => {
  it('listAgentSkills calls GET /a2a/agents/:id/skills (not /agents/:id/skills)', async () => {
    await listAgentSkills('agent1');
    expect(api.get).toHaveBeenCalledWith('/a2a/agents/agent1/skills');
  });

  it('linkSkillToAgent calls POST /a2a/agents/:id/skills', async () => {
    await linkSkillToAgent('agent1', 'skill1');
    expect(api.post).toHaveBeenCalledWith('/a2a/agents/agent1/skills', { skill_id: 'skill1' });
  });

  it('unlinkSkillFromAgent calls DELETE /a2a/agents/:id/skills/:skillId', async () => {
    await unlinkSkillFromAgent('agent1', 'skill1');
    expect(api.delete).toHaveBeenCalledWith('/a2a/agents/agent1/skills/skill1');
  });
});

describe('Runbook Execution API path regressions', () => {
  it('getRunbookExecution calls GET /runbook-executions/:id (not /runbooks/executions/:id)', async () => {
    await getRunbookExecution('exec1');
    expect(api.get).toHaveBeenCalledWith('/runbook-executions/exec1');
  });
});

describe('Knowledge Base API path regressions', () => {
  it('searchKnowledgeBase calls GET /knowledge-base/search with the free-text query (not GET /knowledge-base, which ignores it)', async () => {
    await searchKnowledgeBase({ query: 'connection pool' });
    expect(api.get).toHaveBeenCalledWith('/knowledge-base/search', { params: { query: 'connection pool' } });
  });
});

describe('Runbooks API response-shape regression', () => {
  // GET /runbooks (Go) returns a bare array, never {data,total,...} --
  // listRunbooks was typed as PaginatedResponse<Runbook>, so
  // RunbooksPage's `runbooksData?.data` was always undefined regardless
  // of what the backend returned. Found live validating the Skills
  // registry fix.
  it('listRunbooks resolves to the bare array itself, not array.data', async () => {
    const runbooks = [{ id: 'rb-1', name: 'Restart service' }];
    vi.mocked(api.get).mockResolvedValueOnce({ data: runbooks });

    const result = await listRunbooks();

    expect(result).toBe(runbooks);
    expect((result as any).data).toBeUndefined();
  });
});

describe('Snapshot Config API path regressions', () => {
  it('updateSnapshotConfig calls PUT /observability/snapshots/:id (not nested under sources)', async () => {
    await updateSnapshotConfig('source1', 'cfg1', { name: 'Updated' });
    expect(api.put).toHaveBeenCalledWith('/observability/snapshots/cfg1', { name: 'Updated' });
  });

  it('deleteSnapshotConfig calls DELETE /observability/snapshots/:id', async () => {
    await deleteSnapshotConfig('source1', 'cfg1');
    expect(api.delete).toHaveBeenCalledWith('/observability/snapshots/cfg1');
  });
});

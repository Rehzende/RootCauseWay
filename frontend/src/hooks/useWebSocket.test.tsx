/**
 * Pins a live-found bug: the WS connection URL used to be hardcoded to
 * "localhost:8080", which resolves against the *browser's own* machine, not
 * the server it's actually talking to -- outside local dev (where the
 * frontend and backend happen to share a host), the connection silently
 * never opens. Fixed to build the URL from window.location.host, mirroring
 * how services/api.ts's baseURL: '/api/v1' is already relative/same-origin.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, waitFor } from '@testing-library/react';
import { useWebSocket } from '@/hooks/useWebSocket';

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  url: string;
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  readyState = 0;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }
  send() {}
  close() {
    this.readyState = 3;
  }
}

function TestComponent() {
  useWebSocket();
  return null;
}

describe('useWebSocket', () => {
  beforeEach(() => {
    MockWebSocket.instances = [];
    localStorage.setItem('rootcauseway_token', 'test-token');
    vi.stubGlobal('WebSocket', MockWebSocket as unknown as typeof WebSocket);
  });

  afterEach(() => {
    localStorage.clear();
    vi.unstubAllGlobals();
  });

  it('connects to the current page host, not a hardcoded one', async () => {
    Object.defineProperty(window, 'location', {
      value: { protocol: 'https:', host: 'rootcauseway.rezende.lab' },
      writable: true,
    });

    render(<TestComponent />);

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1));
    const url = MockWebSocket.instances[0].url;

    expect(url.startsWith('wss://rootcauseway.rezende.lab/ws')).toBe(true);
    expect(url).not.toContain('localhost:8080');
  });

  it('uses ws:// (not wss://) over plain http', async () => {
    Object.defineProperty(window, 'location', {
      value: { protocol: 'http:', host: 'localhost:3000' },
      writable: true,
    });

    render(<TestComponent />);

    await waitFor(() => expect(MockWebSocket.instances.length).toBe(1));
    expect(MockWebSocket.instances[0].url.startsWith('ws://localhost:3000/ws')).toBe(true);
  });
});

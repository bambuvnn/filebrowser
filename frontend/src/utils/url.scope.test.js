import { describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/constants.js', () => ({
  globalVars: { baseURL: "/files/" },
}));

vi.mock('@/router/index.ts', () => ({ router: {} }));

vi.mock('@/store/index.ts', () => ({
  state: {
    user: {
      scopes: [
        { name: 'bambuvn', scope: '/Duc' },
        { name: 'othersource', scope: '/' },
      ],
    },
    shareInfo: { isShare: false, hash: '' },
  },
  getters: { isShare: () => false },
  mutations: {},
}));

import { getUserScopeForSource, stripScopeFromPath } from './url.js';

describe('getUserScopeForSource', () => {
  it('returns scope for known source', () => {
    expect(getUserScopeForSource('bambuvn')).toEqual('/Duc');
  });
  it('returns "/" for root-scoped source', () => {
    expect(getUserScopeForSource('othersource')).toEqual('/');
  });
  it('returns "/" for unknown source', () => {
    expect(getUserScopeForSource('nonexistent')).toEqual('/');
  });
});

describe('stripScopeFromPath', () => {
  it('strips scope prefix when path starts with scope', () => {
    expect(stripScopeFromPath('/Duc/video.mp4', 'bambuvn')).toEqual('/video.mp4');
  });
  it('does not strip when scope is "/"', () => {
    expect(stripScopeFromPath('/Duc/video.mp4', 'othersource')).toEqual('/Duc/video.mp4');
  });
  it('does not strip when path does not start with scope', () => {
    expect(stripScopeFromPath('/Other/file.txt', 'bambuvn')).toEqual('/Other/file.txt');
  });
  it('does not strip a partial segment match', () => {
    expect(stripScopeFromPath('/Ducks/file.txt', 'bambuvn')).toEqual('/Ducks/file.txt');
  });
  it('returns "/" when path equals scope exactly', () => {
    expect(stripScopeFromPath('/Duc', 'bambuvn')).toEqual('/');
  });
  it('handles nested paths with scope-like folder names', () => {
    expect(stripScopeFromPath('/Duc/Duc/file.txt', 'bambuvn')).toEqual('/Duc/file.txt');
  });
  it('returns path unchanged for unknown source', () => {
    expect(stripScopeFromPath('/some/path.txt', 'nonexistent')).toEqual('/some/path.txt');
  });
});

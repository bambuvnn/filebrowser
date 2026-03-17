import { describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/constants', () => {
  return {
    globalVars: {
      baseURL: "/files/",
      name: "Test App",
      mediaAvailable: true,
      muPdfAvailable: true,
      onlyOfficeUrl: "",
      recaptcha: false,
      recaptchaKey: "",
      darkMode: false,
      oidcAvailable: false,
      passwordAvailable: true,
      externalUrl: "",
      minSearchLength: 1,
      disableNavButtons: false,
      userSelectableThemes: {},
      enableThumbs: true,
      noAuth: false,
      signup: false,
      version: "test",
      commitSHA: "test",
      disableExternal: false,
      externalLinks: [],
      updateAvailable: "",
    },
    shareInfo: {
      isShare: false,
      disableThumbnails: false,
      hash: "",
      enforceDarkLightMode: "",
      disableSidebar: false,
      isValid: true,
    },
    logoURL: "test-logo.png",
    origin: "http://localhost",
    settings: [],
  };
});

import { removePrefix, extractSourceFromPath, getApiPath, getPublicApiPath, getUserScopeForSource, stripScopeFromPath } from './url.js';

// Mock state for getUserScopeForSource and stripScopeFromPath tests
vi.mock('@/store', () => {
  return {
    state: {
      user: {
        scopes: [
          { name: 'bambuvn', scope: '/Duc' },
          { name: 'othersource', scope: '/' },
        ],
      },
      shareInfo: {
        isShare: false,
        hash: '',
      },
    },
    getters: {
      isShare: () => false,
    },
    mutations: {},
  };
}); (feat: Implement scope-aware path handling for URL generation and interpretation to support multi-user environments.)

describe('testurl', () => {

  it('url prefix', () => {
    const tests = [
      {input: "/files/share/hash", trimArg:"/files/",expected: "/share/hash",},
      {input: "/files/files", trimArg: "/files/",expected: "/files",},
      {input: "/files/share/something/", trimArg: "files", expected:"/share/something/"},
      {input: "test/iscool/", trimArg: "test",expected:"/iscool/"},
      {input: "test",trimArg: "",expected:"/test"},
      {input: "/test", trimArg: "test",expected:"/"},
      {input: "/my/test/file", trimArg: "",expected:"/my/test/file"},
      {input: "/my/test/file", trimArg: "my",expected:"/test/file"},
      {input: "/files/my/test/file", trimArg: "files",expected:"/my/test/file"},
    ]
    for (const test of tests) {
      expect(removePrefix(test.input, test.trimArg)).toEqual(test.expected);
    }
  });

});

describe('getApiPath default', () => {
  it('url prefix', () => {
    const tests = [
      {input: "resources", expected: "/files/api/resources",},
      {input: "share/hash", expected: "/files/api/share/hash",},
      {input: "tools/search", expected: "/files/api/tools/search",},
      {input: "tools/duplicateFinder", expected: "/files/api/tools/duplicateFinder",},
      {input: "tools/fileWatcher", expected: "/files/api/tools/fileWatcher",},
      {input: "tools/fileWatcher/sse", expected: "/files/api/tools/fileWatcher/sse",},
      {input: "office/config", expected: "/files/api/office/config",},
      {input: "office/callback", expected: "/files/api/office/callback",},
      {input: "health", expected: "/files/api/health",},
    ]
    for (const test of tests) {
      expect(getApiPath(test.input)).toEqual(test.expected);
    }
  });
});

describe('getApiPath default with params', () => {
  it('url prefix', () => {
    const tests = [
      {input: "resources", expected: "/files/api/resources?param=resources%20are%20awesome",},
      {input: "share/hash", expected: "/files/api/share/hash?param=resources%20are%20awesome",},
    ]
    for (const test of tests) {
      expect(getApiPath(test.input, { param: "resources are awesome" })).toEqual(test.expected);
    }
  });
});
describe('getApiPath default without encode', () => {
  it('url prefix', () => {
    const tests = [
      {input: "resources", expected: "/files/api/resources?param=resources are awesome",},
    ]
    for (const test of tests) {
      expect(getApiPath(test.input, { param: "resources are awesome" }, true)).toEqual(test.expected);
    }
  });
});

describe('getApiPath public', () => {
  it('url prefix', () => {
    const tests = [
      {input: "resources", expected: "/files/public/api/resources",},
      {input: "office/config", expected: "/files/public/api/office/config",},
      {input: "office/callback", expected: "/files/public/api/office/callback",},
    ]
    for (const test of tests) {
      expect(getPublicApiPath(test.input)).toEqual(test.expected);
    }
  });
});

describe('extractSourceFromPath', () => {
  it('extracts source and path from URL', () => {
    const tests = [
      { url: "/files/default/root/file1.txt", expected: { source: "default", path: "/root/file1.txt" } },
      { url: "/files/default/root/folder1/file1.txt", expected: { source: "default", path: "/root/folder1/file1.txt" } },
      { url: "/files/first/root/file1.txt", expected: { source: "first", path: "/root/file1.txt" } },
      { url: "/files/second/root/folder1/file1.txt", expected: { source: "second", path: "/root/folder1/file1.txt" } },
    ];

    for (const test of tests) {
      const result = extractSourceFromPath(test.url);
      expect(result.source).toEqual(test.expected.source);
      expect(result.path).toEqual(test.expected.path);
    }
  });
});

describe('getUserScopeForSource', () => {
  it('returns scope for known source', () => {
    expect(getUserScopeForSource('bambuvn')).toEqual('/Duc');
  });

  it('returns "/" for admin-like source', () => {
    expect(getUserScopeForSource('othersource')).toEqual('/');
  });

  it('returns "/" for unknown source', () => {
    expect(getUserScopeForSource('nonexistent')).toEqual('/');
  });
});

describe('stripScopeFromPath', () => {
  it('strips scope prefix when path starts with scope', () => {
    // User Duc navigating to absolute URL /Duc/video.mp4
    expect(stripScopeFromPath('/Duc/video.mp4', 'bambuvn')).toEqual('/video.mp4');
  });

  it('does not strip when scope is "/"', () => {
    // Admin navigating to absolute URL
    expect(stripScopeFromPath('/Duc/video.mp4', 'othersource')).toEqual('/Duc/video.mp4');
  });

  it('does not strip when path does not start with scope', () => {
    // Different user's scope doesn't match
    expect(stripScopeFromPath('/Other/file.txt', 'bambuvn')).toEqual('/Other/file.txt');
  });

  it('returns "/" when path equals scope exactly', () => {
    expect(stripScopeFromPath('/Duc', 'bambuvn')).toEqual('/');
  });

  it('handles nested paths with scope-like folder names', () => {
    // User Duc has folder /Duc/Duc/file.txt → stripped to /Duc/file.txt
    expect(stripScopeFromPath('/Duc/Duc/file.txt', 'bambuvn')).toEqual('/Duc/file.txt');
  });

  it('returns path unchanged for unknown source', () => {
    expect(stripScopeFromPath('/some/path.txt', 'nonexistent')).toEqual('/some/path.txt');
  });
});

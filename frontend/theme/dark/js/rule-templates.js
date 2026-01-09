/**
 * Chain Editor 规则模板
 * 这些模板使用统一的 geosite/geoip 语法（由后端适配到实际内核规则）
 */

const RULE_TEMPLATES = {
  // 模板分类
  categories: [
    { id: 'builtin', name: '内置规则', icon: '🛡️' },
    { id: 'streaming', name: '流媒体', icon: '📺' },
    { id: 'social', name: '社交媒体', icon: '💬' },
    { id: 'ai', name: 'AI 服务', icon: '🤖' },
    { id: 'dev', name: '开发工具', icon: '💻' }
  ],

  // 规则模板列表
  templates: [
    // ===== 内置规则 =====
    {
      id: 'cn-direct',
      category: 'builtin',
      name: '国内直连',
      description: '中国大陆域名和 IP 直连',
      icon: '🇨🇳',
      action: 'direct', // direct | proxy | block
      rule: {
        // 使用 geosite/geoip 格式
        domains: ['geosite:cn'],
        ips: ['geoip:cn']
      }
    },
    {
      id: 'private-direct',
      category: 'builtin',
      name: '私有网络直连',
      description: '局域网和私有 IP 地址直连',
      icon: '🏠',
      action: 'direct',
      rule: {
        domains: ['geosite:private'],
        ips: ['geoip:private', '127.0.0.0/8', '10.0.0.0/8', '172.16.0.0/12', '192.168.0.0/16']
      }
    },
    {
      id: 'ads-block',
      category: 'builtin',
      name: '广告拦截',
      description: '拦截常见广告和追踪器',
      icon: '🚫',
      action: 'block',
      rule: {
        domains: ['geosite:category-ads-all']
      }
    },

    // ===== 流媒体 =====
    {
      id: 'youtube',
      category: 'streaming',
      name: 'YouTube',
      description: 'YouTube 视频服务',
      icon: '▶️',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:youtube',
          'domain:youtube.com',
          'domain:googlevideo.com',
          'domain:ytimg.com',
          'domain:ggpht.com'
        ]
      }
    },
    {
      id: 'netflix',
      category: 'streaming',
      name: 'Netflix',
      description: 'Netflix 流媒体服务',
      icon: '🎬',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:netflix',
          'domain:netflix.com',
          'domain:netflix.net',
          'domain:nflximg.net',
          'domain:nflxvideo.net',
          'domain:nflxso.net',
          'domain:nflxext.com'
        ]
      }
    },
    {
      id: 'disney',
      category: 'streaming',
      name: 'Disney+',
      description: 'Disney+ 流媒体服务',
      icon: '🏰',
      action: 'proxy',
      rule: {
        domains: [
          'domain:disney.com',
          'domain:disneyplus.com',
          'domain:disney-plus.net',
          'domain:dssott.com',
          'domain:bamgrid.com',
          'domain:disneystreaming.com'
        ]
      }
    },
    {
      id: 'spotify',
      category: 'streaming',
      name: 'Spotify',
      description: 'Spotify 音乐服务',
      icon: '🎵',
      action: 'proxy',
      rule: {
        domains: [
          'domain:spotify.com',
          'domain:spotifycdn.com',
          'domain:scdn.co',
          'domain:spoti.fi'
        ]
      }
    },
    {
      id: 'twitch',
      category: 'streaming',
      name: 'Twitch',
      description: 'Twitch 直播平台',
      icon: '🎮',
      action: 'proxy',
      rule: {
        domains: [
          'domain:twitch.tv',
          'domain:twitchcdn.net',
          'domain:ttvnw.net',
          'domain:jtvnw.net'
        ]
      }
    },

    // ===== 社交媒体 =====
    {
      id: 'telegram',
      category: 'social',
      name: 'Telegram',
      description: 'Telegram 即时通讯',
      icon: '✈️',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:telegram',
          'domain:telegram.org',
          'domain:telegram.me',
          'domain:t.me',
          'domain:telesco.pe',
          'domain:tdesktop.com'
        ],
        ips: ['geoip:telegram']
      }
    },
    {
      id: 'twitter',
      category: 'social',
      name: 'Twitter/X',
      description: 'Twitter/X 社交平台',
      icon: '🐦',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:twitter',
          'domain:twitter.com',
          'domain:x.com',
          'domain:twimg.com',
          'domain:twittercommunity.com',
          'domain:t.co'
        ]
      }
    },
    {
      id: 'facebook',
      category: 'social',
      name: 'Facebook',
      description: 'Facebook 及 Meta 服务',
      icon: '👤',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:facebook',
          'domain:facebook.com',
          'domain:fb.com',
          'domain:fbcdn.net',
          'domain:fb.me',
          'domain:instagram.com',
          'domain:cdninstagram.com',
          'domain:threads.net'
        ]
      }
    },
    {
      id: 'discord',
      category: 'social',
      name: 'Discord',
      description: 'Discord 语音聊天',
      icon: '🎧',
      action: 'proxy',
      rule: {
        domains: [
          'domain:discord.com',
          'domain:discord.gg',
          'domain:discordapp.com',
          'domain:discordapp.net',
          'domain:discord.media'
        ]
      }
    },
    {
      id: 'reddit',
      category: 'social',
      name: 'Reddit',
      description: 'Reddit 社区',
      icon: '🔴',
      action: 'proxy',
      rule: {
        domains: [
          'domain:reddit.com',
          'domain:redd.it',
          'domain:redditstatic.com',
          'domain:redditmedia.com'
        ]
      }
    },

    // ===== AI 服务 =====
    {
      id: 'openai',
      category: 'ai',
      name: 'OpenAI',
      description: 'ChatGPT 和 OpenAI API',
      icon: '🤖',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:openai',
          'domain:openai.com',
          'domain:chatgpt.com',
          'domain:oaistatic.com',
          'domain:oaiusercontent.com',
          'domain:openaiapi-site.azureedge.net'
        ]
      }
    },
    {
      id: 'anthropic',
      category: 'ai',
      name: 'Claude',
      description: 'Anthropic Claude AI',
      icon: '🧠',
      action: 'proxy',
      rule: {
        domains: [
          'domain:anthropic.com',
          'domain:claude.ai'
        ]
      }
    },
    {
      id: 'google-ai',
      category: 'ai',
      name: 'Google AI',
      description: 'Gemini 和 Google AI 服务',
      icon: '✨',
      action: 'proxy',
      rule: {
        domains: [
          'domain:gemini.google.com',
          'domain:bard.google.com',
          'domain:ai.google.dev',
          'domain:generativelanguage.googleapis.com',
          'domain:aistudio.google.com'
        ]
      }
    },

    // ===== 开发工具 =====
    {
      id: 'google',
      category: 'dev',
      name: 'Google 服务',
      description: 'Google 全系服务',
      icon: '🔍',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:google',
          'domain:google.com',
          'domain:googleapis.com',
          'domain:gstatic.com',
          'domain:googleusercontent.com',
          'domain:googlesyndication.com'
        ]
      }
    },
    {
      id: 'github',
      category: 'dev',
      name: 'GitHub',
      description: 'GitHub 代码托管',
      icon: '🐙',
      action: 'proxy',
      rule: {
        domains: [
          'geosite:github',
          'domain:github.com',
          'domain:github.io',
          'domain:githubusercontent.com',
          'domain:githubstatus.com',
          'domain:githubassets.com'
        ]
      }
    },
    {
      id: 'docker',
      category: 'dev',
      name: 'Docker',
      description: 'Docker Hub 和容器服务',
      icon: '🐳',
      action: 'proxy',
      rule: {
        domains: [
          'domain:docker.com',
          'domain:docker.io',
          'domain:dockerhub.com',
          'domain:gcr.io',
          'domain:ghcr.io',
          'domain:registry.k8s.io'
        ]
      }
    },
    {
      id: 'npm',
      category: 'dev',
      name: 'NPM',
      description: 'NPM 包管理',
      icon: '📦',
      action: 'proxy',
      rule: {
        domains: [
          'domain:npmjs.com',
          'domain:npmjs.org',
          'domain:npmmirror.com',
          'domain:yarnpkg.com'
        ]
      }
    },
    {
      id: 'stackoverflow',
      category: 'dev',
      name: 'Stack Overflow',
      description: 'Stack Overflow 技术问答',
      icon: '📚',
      action: 'proxy',
      rule: {
        domains: [
          'domain:stackoverflow.com',
          'domain:stackexchange.com',
          'domain:sstatic.net',
          'domain:askubuntu.com',
          'domain:serverfault.com',
          'domain:superuser.com'
        ]
      }
    }
  ]
};

// 获取所有模板分类
function getTemplateCategories() {
  return RULE_TEMPLATES.categories;
}

// 获取指定分类的模板
function getTemplatesByCategory(categoryId) {
  if (!categoryId || categoryId === 'all') {
    return RULE_TEMPLATES.templates;
  }
  return RULE_TEMPLATES.templates.filter(t => t.category === categoryId);
}

// 根据 ID 获取模板
function getTemplateById(templateId) {
  return RULE_TEMPLATES.templates.find(t => t.id === templateId);
}

// 搜索模板
function searchTemplates(keyword) {
  if (!keyword) return RULE_TEMPLATES.templates;
  const lower = keyword.toLowerCase();
  return RULE_TEMPLATES.templates.filter(t =>
    t.name.toLowerCase().includes(lower) ||
    t.description.toLowerCase().includes(lower) ||
    t.id.toLowerCase().includes(lower)
  );
}

// 将模板规则转换为 RouteMatchRule 格式
function templateToRouteRule(template) {
  if (!template || !template.rule) return null;
  return {
    domains: template.rule.domains || [],
    ips: template.rule.ips || []
  };
}

// 导出
if (typeof module !== 'undefined' && module.exports) {
  module.exports = {
    RULE_TEMPLATES,
    getTemplateCategories,
    getTemplatesByCategory,
    getTemplateById,
    searchTemplates,
    templateToRouteRule
  };
} else {
  window.RULE_TEMPLATES = RULE_TEMPLATES;
  window.getTemplateCategories = getTemplateCategories;
  window.getTemplatesByCategory = getTemplatesByCategory;
  window.getTemplateById = getTemplateById;
  window.searchTemplates = searchTemplates;
  window.templateToRouteRule = templateToRouteRule;
}

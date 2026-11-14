/**
 * GuardBot WebMCP Tools
 * Exposes tools to AI agents via the WebMCP standard (Chrome 149+).
 *
 * Uses document.modelContext.registerTool() per Chrome's WebMCP Imperative API.
 * https://developer.chrome.com/docs/ai/webmcp/imperative-api
 */
(function(window, document) {
    'use strict';

    var API_ENDPOINT = '/webmcp';
    var RATE_LIMIT = 60; // requests per minute
    var requestCount = 0;
    var requestWindow = null;

    function checkRateLimit() {
        var now = Date.now();
        if (!requestWindow || now - requestWindow > 60000) {
            requestWindow = now;
            requestCount = 0;
        }
        if (requestCount >= RATE_LIMIT) {
            throw new Error('Rate limit exceeded. Max ' + RATE_LIMIT + ' requests per minute.');
        }
        requestCount++;
    }

    function apiCall(endpoint, params, signal) {
        checkRateLimit();

        var url = API_ENDPOINT + endpoint;
        if (params && Object.keys(params).length > 0) {
            url += '?' + new URLSearchParams(params).toString();
        }

        return fetch(url, {
            method: 'GET',
            headers: { 'Accept': 'application/json' },
            credentials: 'same-origin',
            signal: signal
        })
        .then(function(response) {
            if (!response.ok) {
                return response.json().then(function(data) {
                    throw new Error(data.error || 'Request failed');
                });
            }
            return response.json();
        })
        .then(function(data) {
            if (data.success) {
                return data.data;
            }
            throw new Error(data.error || 'Unknown error');
        });
    }

    // Tool definitions
    var tools = [
        {
            name: 'guardbot_search_links',
            description: 'Search your GuardBot redirect links by keyword. Returns matching links with URLs and click counts.',
            inputSchema: {
                type: 'object',
                properties: {
                    q: {
                        type: 'string',
                        description: 'Search query (searches subdomain, path, domain)'
                    },
                    limit: {
                        type: 'number',
                        description: 'Maximum results to return (1-25)'
                    }
                },
                required: ['q']
            },
            annotations: {
                readOnlyHint: true,
                consequentialHint: false
            },
            execute: function(args, options) {
                if (!args.q) {
                    return Promise.reject(new Error('Search query (q) is required'));
                }
                return apiCall('/search', {
                    q: args.q,
                    limit: args.limit || 10
                }, options && options.signal);
            }
        },
        {
            name: 'guardbot_get_domains',
            description: 'List all your verified GuardBot domains with DNS and SSL status.',
            inputSchema: {
                type: 'object',
                properties: {}
            },
            annotations: {
                readOnlyHint: true,
                consequentialHint: false
            },
            execute: function(args, options) {
                return apiCall('/domains', {}, options && options.signal);
            }
        },
        {
            name: 'guardbot_get_links',
            description: 'List your most recent GuardBot redirect links.',
            inputSchema: {
                type: 'object',
                properties: {
                    limit: {
                        type: 'number',
                        description: 'Maximum links to return (1-100)'
                    }
                }
            },
            annotations: {
                readOnlyHint: true,
                consequentialHint: false
            },
            execute: function(args, options) {
                return apiCall('/links', {
                    limit: args.limit || 25
                }, options && options.signal);
            }
        },
        {
            name: 'guardbot_get_link_stats',
            description: 'Get detailed analytics for a specific GuardBot redirect link including click counts, top countries, and device breakdown.',
            inputSchema: {
                type: 'object',
                properties: {
                    link_id: {
                        type: 'string',
                        description: 'The ID of the link to get stats for'
                    }
                },
                required: ['link_id']
            },
            annotations: {
                readOnlyHint: true,
                consequentialHint: false
            },
            execute: function(args, options) {
                if (!args.link_id) {
                    return Promise.reject(new Error('link_id is required'));
                }
                return apiCall('/link-stats', {
                    link_id: args.link_id
                }, options && options.signal);
            }
        }
    ];

    async function registerTools() {
        // Check if WebMCP is supported (Chrome 149+)
        if (!document.modelContext || typeof document.modelContext.registerTool !== 'function') {
            console.log('[WebMCP] document.modelContext.registerTool not available');
            // Fallback: expose tools via window for non-WebMCP agents
            window.GuardBotTools = {
                tools: tools,
                execute: function(name, args) {
                    var tool = tools.find(function(t) { return t.name === name; });
                    if (!tool) return Promise.reject(new Error('Tool not found: ' + name));
                    return tool.execute(args, {});
                }
            };
            return;
        }

        // Register each tool with document.modelContext
        for (var i = 0; i < tools.length; i++) {
            try {
                await document.modelContext.registerTool(tools[i]);
                console.log('[WebMCP] Registered tool:', tools[i].name);
            } catch (e) {
                console.warn('[WebMCP] Failed to register tool:', tools[i].name, e);
            }
        }

        console.log('[WebMCP] GuardBot tools registered');
    }

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', registerTools);
    } else {
        registerTools();
    }

})(window, document);

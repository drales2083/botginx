/**
 * GuardBot WebMCP Bridge
 * Exposes WebMCP tools to autonomous AI agents via browser APIs.
 *
 * AI agents can access these tools through:
 * - navigator.modelContext.tools (Chrome 130+)
 * - window.WebMCP.tools
 * - Document event: 'webmcp:ready'
 */
(function(window, document) {
    'use strict';

    var WebMCP = {
        version: '1.0.0',
        ready: false,
        tools: {},

        config: {
            apiEndpoint: '/webmcp',
            rateLimit: 60, // requests per minute
            enabled: true
        },

        // Rate limiting
        _requestCount: 0,
        _requestWindow: null,

        init: function() {
            if (!this.config.enabled) return;

            this.registerTools();
            this.exposeToNavigator();
            this.emitReadyEvent();

            this.ready = true;
            console.log('[WebMCP] GuardBot tools registered');
        },

        checkRateLimit: function() {
            var now = Date.now();
            if (!this._requestWindow || now - this._requestWindow > 60000) {
                this._requestWindow = now;
                this._requestCount = 0;
            }
            if (this._requestCount >= this.config.rateLimit) {
                throw new Error('Rate limit exceeded. Max ' + this.config.rateLimit + ' requests per minute.');
            }
            this._requestCount++;
        },

        apiCall: function(endpoint, params) {
            var self = this;
            this.checkRateLimit();

            var url = this.config.apiEndpoint + endpoint;
            if (params && Object.keys(params).length > 0) {
                url += '?' + new URLSearchParams(params).toString();
            }

            return fetch(url, {
                method: 'GET',
                headers: {
                    'Accept': 'application/json'
                },
                credentials: 'same-origin'
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
        },

        registerTools: function() {
            var self = this;

            // searchLinks - Search redirect links by keyword
            this.tools.searchLinks = {
                name: 'searchLinks',
                description: 'Search your redirect links by keyword. Returns matching links with their URLs and click counts.',
                inputSchema: {
                    type: 'object',
                    properties: {
                        q: {
                            type: 'string',
                            description: 'Search query (searches subdomain, path, domain)'
                        },
                        limit: {
                            type: 'number',
                            description: 'Maximum results to return (1-25)',
                            default: 10
                        }
                    },
                    required: ['q']
                },
                execute: function(args) {
                    if (!args.q) {
                        return Promise.reject(new Error('Search query (q) is required'));
                    }
                    return self.apiCall('/search', {
                        q: args.q,
                        limit: args.limit || 10
                    });
                }
            };

            // getDomains - List user's domains
            this.tools.getDomains = {
                name: 'getDomains',
                description: 'List all your verified domains with their DNS and SSL status.',
                inputSchema: {
                    type: 'object',
                    properties: {}
                },
                execute: function(args) {
                    return self.apiCall('/domains', {});
                }
            };

            // getLinks - List recent redirect links
            this.tools.getLinks = {
                name: 'getLinks',
                description: 'List your most recent redirect links.',
                inputSchema: {
                    type: 'object',
                    properties: {
                        limit: {
                            type: 'number',
                            description: 'Maximum links to return (1-100)',
                            default: 25
                        }
                    }
                },
                execute: function(args) {
                    return self.apiCall('/links', {
                        limit: args.limit || 25
                    });
                }
            };

            // getLinkStats - Get analytics for a specific link
            this.tools.getLinkStats = {
                name: 'getLinkStats',
                description: 'Get detailed analytics for a specific redirect link including click counts, top countries, and device breakdown.',
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
                execute: function(args) {
                    if (!args.link_id) {
                        return Promise.reject(new Error('link_id is required'));
                    }
                    return self.apiCall('/link-stats', {
                        link_id: args.link_id
                    });
                }
            };
        },

        exposeToNavigator: function() {
            var self = this;

            // Expose via window.WebMCP
            window.WebMCP = this;

            // Chrome 130+ modelContext API
            if (typeof navigator !== 'undefined') {
                try {
                    // Create tools array for modelContext
                    var toolsArray = Object.values(this.tools).map(function(tool) {
                        return {
                            name: tool.name,
                            description: tool.description,
                            inputSchema: tool.inputSchema,
                            execute: tool.execute
                        };
                    });

                    // Register with navigator.modelContext if available
                    if (navigator.modelContext) {
                        if (typeof navigator.modelContext.registerTools === 'function') {
                            navigator.modelContext.registerTools(toolsArray);
                        } else {
                            // Fallback: expose tools directly
                            navigator.modelContext.tools = navigator.modelContext.tools || [];
                            toolsArray.forEach(function(tool) {
                                navigator.modelContext.tools.push(tool);
                            });
                        }
                    } else {
                        // Create modelContext if it doesn't exist
                        Object.defineProperty(navigator, 'modelContext', {
                            value: { tools: toolsArray },
                            writable: false,
                            configurable: true
                        });
                    }
                } catch (e) {
                    console.warn('[WebMCP] Could not register with navigator.modelContext:', e);
                }
            }
        },

        emitReadyEvent: function() {
            var event;
            try {
                event = new CustomEvent('webmcp:ready', {
                    detail: {
                        tools: Object.keys(this.tools),
                        version: this.version
                    }
                });
            } catch (e) {
                // IE fallback
                event = document.createEvent('CustomEvent');
                event.initCustomEvent('webmcp:ready', true, true, {
                    tools: Object.keys(this.tools),
                    version: this.version
                });
            }
            document.dispatchEvent(event);
        },

        // Utility: Get tool by name
        getTool: function(name) {
            return this.tools[name] || null;
        },

        // Utility: Execute tool by name
        executeTool: function(name, args) {
            var tool = this.getTool(name);
            if (!tool) {
                return Promise.reject(new Error('Tool not found: ' + name));
            }
            return tool.execute(args || {});
        },

        // Utility: List available tools
        listTools: function() {
            return Object.values(this.tools).map(function(tool) {
                return {
                    name: tool.name,
                    description: tool.description
                };
            });
        }
    };

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            WebMCP.init();
        });
    } else {
        WebMCP.init();
    }

})(window, document);

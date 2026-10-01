// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://docs.guardbot.sbs',
	redirects: {
		'/': '/guides/introduction/',
	},
	integrations: [
		starlight({
			title: 'GuardBot',
			logo: {
				src: './src/assets/logo.svg',
				replacesTitle: true,
			},
			favicon: '/favicon.ico',
			head: [
				{
					tag: 'link',
					attrs: {
						rel: 'icon',
						type: 'image/png',
						sizes: '32x32',
						href: '/favicon-32x32.png',
					},
				},
			],
			customCss: ['./src/styles/custom.css'],
			social: [
				{ icon: 'external', label: 'Main Site', href: 'https://guardbot.sbs' },
				{ icon: 'email', label: 'Support', href: 'https://guardbot.sbs/user/support' },
			],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'guides/introduction' },
						{ label: 'Quick Start', slug: 'guides/quick-start' },
					],
				},
				{
					label: 'Features',
					items: [
						{ label: 'Redirect Links', slug: 'features/redirect-links' },
						{ label: 'Bot Protection', slug: 'features/bot-protection' },
						{ label: 'Hosting', slug: 'features/hosting' },
						{ label: 'Analytics', slug: 'features/analytics' },
						{ label: 'QR Codes', slug: 'features/qr-codes' },
					],
				},
				{
					label: 'Integrations',
					items: [
						{ label: 'Telegram Bot', slug: 'integrations/telegram' },
						{ label: 'Payments', slug: 'integrations/payments' },
					],
				},
				{
					label: 'API Reference',
					items: [
						{ label: 'Overview', slug: 'api/overview' },
						{ label: 'Authentication', slug: 'api/authentication' },
						{ label: 'Rate Limiting', slug: 'api/rate-limiting' },
						{ label: 'Errors', slug: 'api/errors' },
					],
				},
				{
					label: 'API Endpoints',
					items: [
						{ label: 'Links', slug: 'api/links' },
						{ label: 'Domains', slug: 'api/domains' },
						{ label: 'Analytics', slug: 'api/analytics' },
						{ label: 'QR Codes', slug: 'api/qrcodes' },
						{ label: 'Shortener', slug: 'api/shortener' },
						{ label: 'IP Lists', slug: 'api/iplists' },
						{ label: 'Account', slug: 'api/account' },
						{ label: 'Webhooks', slug: 'api/webhooks' },
					],
				},
			],
		}),
	],
});

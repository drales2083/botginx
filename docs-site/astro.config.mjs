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
			components: {
				SocialIcons: './src/components/SocialIcons.astro',
			},
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'What is GuardBot?', slug: 'guides/introduction' },
						{ label: 'Tutorial: First Link', slug: 'guides/quick-start' },
					],
				},
				{
					label: 'Concepts',
					items: [
						{ label: 'Redirect Links', slug: 'features/redirect-links' },
						{ label: 'Bot Protection', slug: 'features/bot-protection' },
						{ label: 'Hosting', slug: 'features/hosting' },
						{ label: 'Analytics', slug: 'features/analytics' },
						{ label: 'QR Codes', slug: 'features/qr-codes' },
					],
				},
				{
					label: 'How-to Guides',
					items: [
						{ label: 'Connect cPanel', slug: 'integrations/cpanel' },
						{ label: 'Connect NameSilo', slug: 'integrations/namesilo' },
						{ label: 'Connect Telegram', slug: 'integrations/telegram' },
						{ label: 'Add Funds', slug: 'integrations/payments' },
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

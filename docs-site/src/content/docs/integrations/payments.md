---
title: Payments
description: Add funds to your account using Bitcoin.
---

GuardBot runs on account balance. You deposit crypto, it converts to USD credit, you spend it on hosting and features. No credit cards, no chargebacks, no payment processor hassles.

## Supported Currencies

| Currency | Network | Minimum Deposit |
|----------|---------|-----------------|
| Bitcoin (BTC) | Bitcoin mainnet | 0.0001 BTC |

More currencies coming. For now, Bitcoin is the only option.

## Making a Deposit

### Step 1: Go to Deposit

Navigate to **Payments** → **Deposit** in your dashboard.

### Step 2: Generate an Address

Click **Generate Address**. You'll get a Bitcoin address unique to your account.

This address is yours permanently. You can reuse it for future deposits or generate a new one if you prefer.

### Step 3: Send Bitcoin

Send Bitcoin from your wallet to the address shown. Double-check the address before confirming. Crypto transactions can't be reversed.

### Step 4: Wait for Confirmation

Your deposit shows as "Pending" immediately after the transaction broadcasts to the network.

After 1 confirmation (roughly 10 minutes), the funds credit to your balance. You'll see the USD amount based on the exchange rate at confirmation time.

## Viewing Transactions

Go to **Payments** → **Transactions** to see your history:

- All crypto deposits with status
- Balance changes (purchases, refunds, commissions)
- Transaction timestamps and amounts

### Transaction Statuses

| Status | Meaning |
|--------|---------|
| Pending | Waiting for network confirmation |
| Confirmed | Credited to your balance |
| Failed | Something went wrong (rare) |

## Using Your Balance

Balance pays for:

- **Hosting packages**: Monthly fees deducted automatically
- **Subscription renewal**: Keep your account active
- **Premium features**: Anything with a price tag

Charges pull from your balance. If you don't have enough, the purchase fails. Keep your balance funded to avoid service interruption.

## Exchange Rates

Deposits convert to USD at the market rate when the transaction confirms. The rate comes from CoinGecko.

You see the exact USD amount credited. No hidden fees on the conversion.

## Minimum Deposit

The minimum is 0.0001 BTC, roughly a few dollars depending on the current price. Deposits below the minimum don't credit to your account.

## Refunds

Crypto deposits can't be refunded. Only deposit what you intend to use.

If you deposit by mistake (sent too much, sent to wrong account), contact support. We'll look into it, but no guarantees.

## Security

Your deposit address is generated through BitGo, a regulated custody provider. Funds go directly to your account. We don't hold large amounts in hot wallets.

The address is unique to you. Don't share it publicly unless you want random people sending you money (which, sure, fine, but weird).

## FAQ

**How long until my deposit shows?**

The transaction appears as pending immediately after broadcast. It credits after 1 network confirmation, typically 10 minutes for Bitcoin.

**Is there a maximum deposit?**

No maximum, but large deposits may take longer to confirm depending on network congestion.

**What if I sent to the wrong address?**

If you sent to someone else's address, the funds are gone. Triple-check addresses before sending.

**Can I withdraw my balance?**

Currently no. Balance is for spending on GuardBot services. Withdrawal functionality may come later.

**What happens if the price changes after I deposit?**

The conversion rate locks at confirmation time. Price changes afterward don't affect your credited balance.

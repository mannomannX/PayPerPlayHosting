# Business-Modell & Kalkulation

## Geschäftsmodell-Übersicht

**Kernkonzept**: Minecraft-Server-Hosting nach tatsächlicher Nutzung abrechnen, nicht pauschal monatlich.

**USP (Unique Selling Proposition)**:
- Zahle nur, wenn du spielst
- Keine verschwendeten Ressourcen bei Inaktivität
- Flexible Server-Konfiguration (RAM, Slots, Mods)
- Unterstützung aller MC-Versionen (1.8 - neueste)

## Pricing-Strategie

### Option A: Reine Pay-Per-Play (einfach, transparent)

```
Server-Größen:
┌─────────────┬──────────┬─────────────┬───────────────┐
│ Plan        │ RAM      │ Max Spieler │ €/Stunde      │
├─────────────┼──────────┼─────────────┼───────────────┤
│ Starter     │ 2 GB     │ 1-5         │ €0,10         │
│ Standard    │ 4 GB     │ 6-15        │ €0,20         │
│ Premium     │ 8 GB     │ 16-30       │ €0,40         │
│ Performance │ 16 GB    │ 31-60       │ €0,80         │
│ Enterprise  │ 32 GB    │ 61+         │ €1,50         │
└─────────────┴──────────┴─────────────┴───────────────┘

Besonderheiten:
- Erste 10 Minuten pro Start kostenlos (Cold-Start-Bonus)
- Abrechnung sekundengenau
- Monatliches Ausgabenlimit einstellbar
```

### Option B: Hybrid-Modell (empfohlen!)

```
Basis-Preise (inkl. Inklusiv-Stunden):
┌─────────────┬──────────┬────────────┬──────────────┬────────────┐
│ Plan        │ RAM      │ Grundpreis │ Inkl. Std.   │ €/Std extra│
├─────────────┼──────────┼────────────┼──────────────┼────────────┤
│ Starter     │ 2 GB     │ €2,99/Mon  │ 10h          │ €0,15      │
│ Standard    │ 4 GB     │ €4,99/Mon  │ 15h          │ €0,20      │
│ Premium     │ 8 GB     │ €9,99/Mon  │ 20h          │ €0,30      │
│ Performance │ 16 GB    │ €19,99/Mon │ 25h          │ €0,50      │
└─────────────┴──────────┴────────────┴──────────────┴────────────┘

Add-Ons:
┌──────────────────────────┬──────────────┐
│ Feature                  │ Preis        │
├──────────────────────────┼──────────────┤
│ Always-On (24/7)         │ +€8,00/Mon   │
│ Tägliche Backups (7d)    │ +€1,50/Mon   │
│ Premium-Support (1h SLA) │ +€4,99/Mon   │
│ Dedizierte IPv4          │ +€2,00/Mon   │
│ Custom Subdomain         │ +€0,99/Mon   │
│ Mod-Pack-Installation    │ +€0,00 (inkl)│
└──────────────────────────┴──────────────┘
```

### Gewinnmargen-Analyse

#### Kostenstruktur (pro Dedicated Server)

**Hetzner AX41-NVMe (~€40/Monat)**:
```
Hardware-Kapazität:
- 64 GB RAM → ~12x 4GB-Server gleichzeitig möglich
- 512 GB NVMe → ~50x 10GB-Welten speicherbar
- CPU → ~20-30 MC-Server mit guter Performance

Annahme: Durchschnittlich 30% Gleichzeitigkeits-Rate
→ Verkaufe bis zu 40x 4GB-Slots auf einem Server
```

**Zusatzkosten**:
- Hetzner Storage Box (1 TB): €5/Monat
- Lizenz-Kosten (Pterodactyl Pro etc.): €10/Monat
- Strom/Traffic: inkludiert
- **Total: ~€55/Monat pro physischem Server**

#### Umsatz-Berechnung (konservativ)

**Szenario: 40 Kunden auf einem Dedicated Server**

```
Annahmen:
- 40 Kunden mit "Standard"-Plan (€4,99/Mon)
- Durchschnittliche Zusatz-Nutzung: 10h/Monat à €0,20/h
- 20% buchen Add-Ons (Ø €3/Mon pro Kunde)

Kalkulation:
Base-Revenue:      40 × €4,99  = €199,60
Usage-Revenue:     40 × €2,00  = €80,00
Add-On-Revenue:    8 × €3,00   = €24,00
─────────────────────────────────────────
TOTAL REVENUE:                  €303,60/Mon

Kosten:                         -€55,00/Mon
Support (10h à €20/h):          -€200,00/Mon (deine Zeit)
─────────────────────────────────────────
GEWINN:                         €48,60/Mon (88% Overhead!)
```

**Optimiertes Szenario** (mit Automatisierung):
```
- Gleiche 40 Kunden
- Support reduziert auf 2h/Mon durch gute Docs/Automation
- Support-Kosten: -€40/Mon

GEWINN: €208,60/Mon (79% Margin!)
```

#### Skalierungs-Projektion

**Jahr 1** (Start):
```
Monat 1-3:   1 Server, 10-20 Kunden    → -€100 bis +€50 (Aufbau)
Monat 4-6:   1 Server, 30-40 Kunden    → +€150 bis +€250
Monat 7-12:  2 Server, 60-80 Kunden    → +€400 bis +€600
──────────────────────────────────────────────────────────
Jahr 1 Gesamt-Gewinn:                   ~€2.500-€3.500
```

**Jahr 2** (Wachstum):
```
3-5 Dedicated Server
150-200 Kunden
Monatlicher Gewinn: €1.500-€2.500
Jahres-Gewinn: ~€20.000-€30.000
```

## Konkurrenz-Analyse

### Vergleich mit bestehenden Anbietern

| Anbieter | Modell | 4GB-Server-Preis | Nachteil |
|----------|--------|------------------|----------|
| **Nitrado** | Pauschal | €13/Mon | Teuer, auch bei Nichtnutzung |
| **G-Portal** | Pauschal | €10/Mon | Keine flexible Skalierung |
| **Aternos** | Kostenlos | €0 | Werbung, Warteschlangen, langsam |
| **Minehut** | Freemium | €0-€8 | Limitierte Plugins, schlechte Performance |
| **Pebblehost** | Pauschal | $5/Mon | Nur US-Hosting |
| **DU (PayPerPlay)** | Pay-per-Use | €4,99 + Usage | ✓ Fair, schnell, DE-Hosting |

### Wettbewerbsvorteile

1. **Preis-Transparenz**: Echte Pay-per-Use, keine versteckten Kosten
2. **Performance**: Hetzner-Hardware, optimierte JVM-Flags
3. **Flexibilität**: Alle MC-Versionen + Mods/Plugins
4. **DSGVO**: Deutsches Hosting, EU-Compliance
5. **Support**: Deutscher Support, Community-Discord

## Marketing-Strategie

### Zielgruppe

**Primär**:
- Hobby-Spieler (16-25 Jahre)
- Kleine Freundesgruppen (3-10 Spieler)
- Projekt-basierte Server (z.B. für YouTube-Serien)

**Sekundär**:
- Mod-Pack-Entwickler (Testsysteme)
- Event-Server (z.B. für Conventions)
- Schulen/Bildungseinrichtungen (Minecraft Education)

### Akquise-Kanäle

**Organisch** (kostenlos, aber zeitintensiv):
1. **Reddit**: r/admincraft, r/minecraft, r/de_EDV
   - Hilfreiche Posts, keine direkte Werbung
   - "Ich habe ein Pay-per-Play-Hosting gebaut, AMA"
2. **Discord**: MC-Community-Server joinen
   - Support in #server-help-Channels geben
   - Subtil auf eigenes Angebot hinweisen
3. **YouTube**: Tutorials "Minecraft Server erstellen 2025"
   - Vergleich: Klassisches Hosting vs. Pay-per-Play
4. **Forum**: SpigotMC, PaperMC-Forums
   - Signatur mit Link
   - In Support-Threads helfen

**Bezahlt** (wenn Budget >€500/Mon):
1. **Google Ads**: "Minecraft Server mieten" (€0,50-€2 CPC)
2. **YouTuber-Sponsorships**: Kleine MC-YouTuber (10k-50k Subs)
   - €50-€200 per Video
3. **Reddit Ads**: Zielgruppe r/minecraft (günstig)

### Launch-Strategie

**Beta-Phase** (Monat 1-2):
- 10-20 Beta-Tester kostenlos
- Feedback sammeln
- Bugs fixen
- Testimonials sammeln für Website

**Soft-Launch** (Monat 3):
- Website live
- Pricing 20% Rabatt für ersten Monat
- Reddit/Discord-Posts
- "Pay what you want"-Option für erste 50 Kunden

**Full-Launch** (Monat 4+):
- Volle Preise
- Referral-Programm (€2 Guthaben pro Neukunden-Werbung)
- Content-Marketing (Blog-Posts, Tutorials)

## Retention-Strategie

### Kundenbindung

**Onboarding**:
- Interaktive Tutorials im Panel
- Erster Server-Setup in <5 Minuten
- Welcome-E-Mail mit Best Practices

**Engagement**:
- Monatliche Newsletter mit MC-News
- Community-Events (z.B. Server-Wettbewerbe)
- Discord-Community mit Support + Off-Topic

**Incentives**:
- Treueprogramm: Nach 6 Monaten 10% Rabatt
- Guthaben-Bonuses bei Aufladung (z.B. €50 aufladen → €55 Guthaben)
- Referral-Rewards

### Churn-Prevention

**Typische Kündigungs-Gründe**:
1. Server wird nicht mehr genutzt → Automatische Pausierung nach 30d Inaktivität
2. Technische Probleme → Proaktiver Support bei Abstürzen
3. Konkurrenz günstiger → Preisanpassungs-Garantie
4. Feature fehlt → Feature-Request-Board, schnelle Implementierung

**Rückgewinnungs-Kampagnen**:
- E-Mail nach 30d Inaktivität: "Wir vermissen dich! Hier ist €2 Guthaben"
- Survey: Warum hast du aufgehört? (Incentive: €1 Guthaben für Feedback)

## Rechtliche & Compliance

### Geschäftsform
- **Einzelunternehmen** (Start) → Gewerbe anmelden
- **GmbH** (ab ~€50k Umsatz/Jahr) → Haftungsbeschutz

### DSGVO-Konformität
- Hosting in Deutschland (Hetzner)
- Privacy Policy auf Website
- AGB mit Klausel für Server-Inhalte
- Opt-In für Marketing-E-Mails
- Recht auf Datenlöschung (DSGVO Art. 17)

### AGB-Punkte (wichtig!)
- Keine Haftung für Spieler-generierte Inhalte
- Backup-Retention nur 30 Tage
- Kündigungsfrist: 14 Tage (bei Monats-Abo)
- Fair-Use-Policy (gegen Missbrauch, z.B. Mining)
- Minecraft EULA-Compliance (keine Pay-to-Win-Features)

### Steuern
- Umsatzsteuer: 19% (Deutschland)
- Kleinunternehmer-Regelung bis €22.000 Umsatz/Jahr
- Einkommensteuer: Progressiv (dein persönlicher Steuersatz)

## Exit-Strategie (langfristig)

**Optionen bei erfolgreichem Wachstum**:
1. **Verkauf** an größeren Hosting-Anbieter (z.B. Nitrado, G-Portal)
   - Bewertung: ~1-3x Jahresumsatz bei profitablen SaaS
2. **Weiterführen** als passives Einkommen (mit Automation)
3. **Merger** mit komplementärem Service (z.B. Discord-Bot-Hosting)

**Wann ist guter Zeitpunkt?**
- Bei >1.000 Kunden und stabiler Profit-Margin
- Wenn Skalierung zu komplex wird (Multi-Region etc.)
- Persönliche Gründe (neues Projekt, keine Zeit mehr)

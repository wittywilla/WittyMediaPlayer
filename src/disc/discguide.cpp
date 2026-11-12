// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
#include "discguide.h"

#include "../ui/controlbar.h"

#include <QJsonArray>
#include <cmath>

namespace wmp {

namespace {

DiscChoice choiceFrom(const QJsonObject &o)
{
    DiscChoice c;
    c.id = o.value(QStringLiteral("id")).toString();
    c.kind = o.value(QStringLiteral("kind")).toString();
    c.label = o.value(QStringLiteral("label")).toString();
    c.detail = o.value(QStringLiteral("detail")).toString();
    c.title = o.value(QStringLiteral("title")).toInt();
    const QJsonArray titles = o.value(QStringLiteral("titles")).toArray();
    for (int i = 0; i < titles.size() && i < DiscGuide::kMaxChoices; i++)
        c.titles.push_back(titles[i].toInt());
    c.duration = o.value(QStringLiteral("durationSec")).toDouble();
    c.chapters = o.value(QStringLiteral("chapters")).toInt();
    c.startSec = o.value(QStringLiteral("startSec")).toDouble();
    c.main = o.value(QStringLiteral("main")).toBool();
    c.oneStream = o.value(QStringLiteral("oneStream")).toBool();
    c.source = o.value(QStringLiteral("source")).toString();
    // A length that isn't a time is no length.
    if (!std::isfinite(c.duration) || c.duration < 0)
        c.duration = 0;
    if (!std::isfinite(c.startSec) || c.startSec < 0)
        c.startSec = 0;
    return c;
}

QVector<DiscChoice> choicesFrom(const QJsonValue &v)
{
    QVector<DiscChoice> out;
    const QJsonArray a = v.toArray();
    for (int i = 0; i < a.size() && out.size() < DiscGuide::kMaxChoices; i++) {
        const DiscChoice c = choiceFrom(a[i].toObject());
        if (c.valid())
            out.push_back(c);
    }
    return out;
}

QStringList linesFrom(const QJsonValue &v)
{
    QStringList out;
    const QJsonArray a = v.toArray();
    for (int i = 0; i < a.size() && out.size() < DiscGuide::kMaxNotes; i++) {
        const QString s = a[i].toString().trimmed();
        if (!s.isEmpty())
            out << s;
    }
    return out;
}

} // namespace

QString DiscChoice::menuText() const
{
    QString text = label;
    if (duration > 0)
        text += QStringLiteral(", %1").arg(ControlBar::formatTime(duration));
    if (main)
        text += QStringLiteral(" (main feature)");
    else if (!detail.isEmpty())
        text += QStringLiteral(" (%1)").arg(detail);
    // A menu reads "&" as the mark of a shortcut key.
    return text.replace(QLatin1Char('&'), QStringLiteral("&&"));
}

DiscGuide DiscGuide::fromJson(const QJsonObject &o)
{
    DiscGuide g;
    if (o.isEmpty())
        return g;
    g.present = true;
    g.raw = o;
    g.kind = o.value(QStringLiteral("kind")).toString();
    g.feature = choiceFrom(o.value(QStringLiteral("feature")).toObject());
    g.why = o.value(QStringLiteral("why")).toString().trimmed();
    g.reasons = linesFrom(o.value(QStringLiteral("reasons")));
    g.confidence = qBound(0.0, o.value(QStringLiteral("confidence")).toDouble(), 1.0);
    g.level = o.value(QStringLiteral("level")).toString();
    if (g.level != QLatin1String("high") && g.level != QLatin1String("medium"))
        g.level = QStringLiteral("low");
    g.versions = choicesFrom(o.value(QStringLiteral("versions")));
    g.episodes = choicesFrom(o.value(QStringLiteral("episodes")));
    g.others = choicesFrom(o.value(QStringLiteral("others")));
    g.playAll = choiceFrom(o.value(QStringLiteral("playAll")).toObject());
    g.decoys = qMax(0, o.value(QStringLiteral("decoys")).toInt());
    g.hidden = qMax(0, o.value(QStringLiteral("hidden")).toInt());
    g.warnings = linesFrom(o.value(QStringLiteral("warnings")));
    g.repairs = linesFrom(o.value(QStringLiteral("repairs")));
    g.repairCount = qMax(int(g.repairs.size()), o.value(QStringLiteral("repairCount")).toInt());
    g.paddedSectors = qMax(qint64(0), qint64(o.value(QStringLiteral("paddedSectors")).toDouble()));
    g.readRetries = qMax(qint64(0), qint64(o.value(QStringLiteral("readRetries")).toDouble()));
    return g;
}

QString DiscGuide::unit() const
{
    return kind == QLatin1String("bluray") ? QStringLiteral("playlist") : QStringLiteral("title");
}

QString DiscGuide::versionsName() const
{
    return kind == QLatin1String("bluray") ? QStringLiteral("Editions") : QStringLiteral("Versions");
}

bool DiscGuide::lists(int title) const
{
    if (feature.valid() && (feature.title == title || feature.titles.contains(title)))
        return true;
    if (playAll.valid() && playAll.title == title)
        return true;
    for (const auto *list : {&versions, &episodes, &others})
        for (const DiscChoice &c : *list)
            if (c.title == title)
                return true;
    return false;
}

DiscChoice DiscGuide::find(const QString &id) const
{
    if (id.isEmpty())
        return {};
    if (feature.id == id)
        return feature;
    if (playAll.id == id)
        return playAll;
    for (const auto *list : {&versions, &episodes, &others})
        for (const DiscChoice &c : *list)
            if (c.id == id)
                return c;
    return {};
}

QString DiscGuide::detailFor(int title) const
{
    if (feature.valid() && feature.titles.size() > 1 && feature.titles.contains(title))
        return QStringLiteral("part %1 of the main feature").arg(feature.titles.indexOf(title) + 1);
    for (const DiscChoice &c : others)
        if (c.title == title)
            return c.detail;
    return {};
}

QString DiscGuide::summary() const
{
    if (!present || !feature.valid())
        return {};
    QString s = QStringLiteral("The main feature is %1, %2.").arg(feature.label.left(1).toLower() + feature.label.mid(1), ControlBar::formatTime(feature.duration));
    if (level == QLatin1String("high"))
        s += QStringLiteral(" The disc engine is sure of it.");
    else if (level == QLatin1String("medium"))
        s += QStringLiteral(" The disc engine is fairly sure of it.");
    else
        s += QStringLiteral(" The disc engine isn't sure of it: see Disc, Why this %1.").arg(unit());
    return s;
}

} // namespace wmp

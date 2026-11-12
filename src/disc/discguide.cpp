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

QString counted(qint64 n, const QString &one, const QString &many)
{
    return n == 1 ? QStringLiteral("1 %1").arg(one) : QStringLiteral("%1 %2").arg(n).arg(many);
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

QString DiscGuide::whyText() const
{
    if (!present)
        return QStringLiteral("This disc's module is from before it could say why it picks a title. A newer one may be in File, Settings.");
    if (!feature.valid())
        return QStringLiteral("The disc engine found nothing on this disc that could be its main feature.");
    QStringList out;
    out << (why.isEmpty() ? QStringLiteral("%1 was picked as the main feature.").arg(feature.label) : why);
    out << QStringLiteral("How sure: %1 (%2%).").arg(level).arg(qRound(confidence * 100));
    QString length = QStringLiteral("Length: %1").arg(ControlBar::formatTime(feature.duration));
    if (feature.chapters > 0)
        length += QStringLiteral(", %1").arg(counted(feature.chapters, QStringLiteral("chapter"), QStringLiteral("chapters")));
    out << length + QLatin1Char('.');
    if (feature.titles.size() > 1) {
        QStringList nums;
        for (int t : feature.titles)
            nums << QString::number(t);
        out << (feature.oneStream
                    ? QStringLiteral("The disc cuts the film into %1 titles (%2). They play here as one film, in the disc's order.").arg(feature.titles.size()).arg(nums.join(QStringLiteral(", ")))
                    : QStringLiteral("The disc cuts the film into %1 titles (%2), and its own program plays them one after another.").arg(feature.titles.size()).arg(nums.join(QStringLiteral(", "))));
    } else if (feature.source == QLatin1String("commands")) {
        out << QStringLiteral("The film is put together by the disc's own commands, and plays here in that order.");
    }
    if (!reasons.isEmpty())
        out << QString() << reasons;
    if (!warnings.isEmpty())
        out << QString() << warnings;
    QStringList facts;
    if (decoys > 0)
        facts << QStringLiteral("%1 left out of the menus; Disc, Titles, More titles still has them.").arg(counted(decoys, QStringLiteral("decoy %1 is").arg(unit()), QStringLiteral("decoy %1s are").arg(unit())));
    if (repairCount > 0) {
        facts << (kind == QLatin1String("bluray")
                      ? QStringLiteral("The disc's playlists, clip information and stream files disagree in %1:").arg(counted(repairCount, QStringLiteral("place"), QStringLiteral("places")))
                      : QStringLiteral("The disc's tables disagreed in %1:").arg(counted(repairCount, QStringLiteral("place"), QStringLiteral("places"))));
        const int shown = qMin(4, int(repairs.size()));
        for (int i = 0; i < shown; i++)
            facts << QStringLiteral("  %1").arg(repairs[i]);
        if (repairCount > shown)
            facts << QStringLiteral("  and %1 more.").arg(repairCount - shown);
    }
    if (paddedSectors > 0)
        facts << QStringLiteral("So far %1 of the disc couldn't be read and %2 taken as empty.").arg(counted(paddedSectors, QStringLiteral("sector"), QStringLiteral("sectors")), paddedSectors == 1 ? QStringLiteral("was") : QStringLiteral("were"));
    if (!facts.isEmpty())
        out << QString() << facts;
    out << QString() << QStringLiteral("This comes from reading the disc's index, never from unlocking anything: a copy-protected disc is refused.");
    return out.join(QLatin1Char('\n'));
}

} // namespace wmp

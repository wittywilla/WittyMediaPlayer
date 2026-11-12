// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
#include "discmemory.h"

#include <QDateTime>
#include <QSettings>
#include <algorithm>
#include <utility>
#include <vector>

namespace wmp::DiscMemory {

namespace {
const QString kGroup = QStringLiteral("discFilm");
// Labels come from a disc; a menu line is enough of one.
constexpr int kMaxLabel = 80;
} // namespace

bool validFingerprint(const QString &fp)
{
    if (fp.size() != 32)
        return false;
    return std::all_of(fp.begin(), fp.end(), [](QChar c) { return (c >= QLatin1Char('0') && c <= QLatin1Char('9')) || (c >= QLatin1Char('a') && c <= QLatin1Char('f')); });
}

DiscPick load(QSettings &s, const QString &fp)
{
    DiscPick p;
    if (!validFingerprint(fp))
        return p;
    s.beginGroup(kGroup);
    s.beginGroup(fp);
    p.id = s.value(QStringLiteral("id")).toString().left(64);
    p.title = qMax(0, s.value(QStringLiteral("title")).toInt());
    p.label = s.value(QStringLiteral("label")).toString().left(kMaxLabel);
    s.endGroup();
    s.endGroup();
    return p;
}

void save(QSettings &s, const QString &fp, const DiscPick &pick)
{
    if (!validFingerprint(fp) || !pick.valid())
        return;
    s.beginGroup(kGroup);
    s.beginGroup(fp);
    s.setValue(QStringLiteral("id"), pick.id.left(64));
    s.setValue(QStringLiteral("title"), pick.title);
    s.setValue(QStringLiteral("label"), pick.label.left(kMaxLabel));
    s.setValue(QStringLiteral("when"), QDateTime::currentSecsSinceEpoch());
    s.endGroup();
    // A viewer with a library of discs keeps the newest choices.
    const QStringList discs = s.childGroups();
    if (discs.size() > kMaxDiscs) {
        std::vector<std::pair<qint64, QString>> byAge;
        for (const QString &d : discs)
            byAge.emplace_back(s.value(d + QStringLiteral("/when")).toLongLong(), d);
        std::sort(byAge.begin(), byAge.end());
        for (int i = 0; i < int(byAge.size()) - kMaxDiscs; i++)
            s.remove(byAge[i].second);
    }
    s.endGroup();
}

void forget(QSettings &s, const QString &fp)
{
    if (!validFingerprint(fp))
        return;
    s.beginGroup(kGroup);
    s.remove(fp);
    s.endGroup();
}

} // namespace wmp::DiscMemory

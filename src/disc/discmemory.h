// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// The film a viewer chose for a disc, instead of the one the disc engine picked: kept on
// this computer, in the player's settings, by the disc's fingerprint (which the helper
// works out from the disc's index). Nothing about the disc but that name and the choice
// is kept.
#pragma once

#include <QString>

class QSettings;

namespace wmp {

struct DiscPick {
    QString id;     // the guide's id for it, or empty for a title the guide doesn't list
    int title = 0;  // the title or playlist that plays it
    QString label;  // what the menus call it: "Extended cut", "Title 4"

    bool valid() const { return title > 0; }
};

namespace DiscMemory {

// How many discs' choices are kept; the oldest go first.
constexpr int kMaxDiscs = 500;

// Whether a fingerprint is one the helper makes: 32 hexadecimal digits.
bool validFingerprint(const QString &fingerprint);
DiscPick load(QSettings &settings, const QString &fingerprint);
void save(QSettings &settings, const QString &fingerprint, const DiscPick &pick);
void forget(QSettings &settings, const QString &fingerprint);

} // namespace DiscMemory

} // namespace wmp

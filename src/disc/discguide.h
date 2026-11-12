// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// The disc engine's guide to an open disc, as the helper sends it with "open": which
// title (DVD) or playlist (Blu-ray) it picked as the film, why, how sure it is, and what
// else there is to choose. WittyDVD and WittyBluray send the same shape.
#pragma once

#include <QJsonObject>
#include <QString>
#include <QStringList>
#include <QVector>

namespace wmp {

// One thing the viewer can ask the engine to play, by its id.
struct DiscChoice {
    QString id;
    QString kind; // feature, version, episode, playAll or title
    QString label, detail;
    int title = 0;       // the title or playlist that plays it
    QVector<int> titles; // all of them, when a disc cuts one film into several titles
    double duration = 0;
    int chapters = 0;
    double startSec = 0; // where it begins inside its title
    bool main = false;   // the version that is the film as picked
    bool oneStream = false;
    QString source;

    bool valid() const { return !id.isEmpty(); }
    // "Episode 3, 21:36 (in title 1, from 0:43:30)": a line for a menu.
    QString menuText() const;
};

struct DiscGuide {
    // Lists longer than this are cut: a guide is a menu, not a catalogue.
    static constexpr int kMaxChoices = 100;
    static constexpr int kMaxNotes = 16;

    QJsonObject raw; // the guide as the module sent it: the pages are made from it
    bool present = false; // an older disc module sends none
    QString kind;         // dvd or bluray
    DiscChoice feature;
    QString why;
    QStringList reasons;
    double confidence = 0;
    QString level; // high, medium or low
    QVector<DiscChoice> versions, episodes, others;
    DiscChoice playAll;
    int decoys = 0, hidden = 0;
    QStringList warnings, repairs;
    int repairCount = 0;
    qint64 paddedSectors = 0, readRetries = 0;

    static DiscGuide fromJson(const QJsonObject &o);

    // "title" on a DVD, "playlist" on a Blu-ray.
    QString unit() const;
    // What a disc's other cuts are called: "Editions" on a Blu-ray, "Versions" on a DVD.
    QString versionsName() const;
    // Whether a title or playlist is one the guide lists by itself, not a decoy or one
    // past its limits.
    bool lists(int title) const;
    // The choice with this id, or an invalid one.
    DiscChoice find(const QString &id) const;
    // What the guide says about a title, for the Titles menu: "trailer", "episode".
    QString detailFor(int title) const;
    // One line: the pick, its length and how sure. Read out when a disc opens.
    QString summary() const;
};

} // namespace wmp

// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 WittyWilla and contributors
// The window's side of the SDUI pages: the state it tells the engine, and what an action on a
// page does. The actions are the list in engine/internal/pages (Actions); a test checks the two
// agree. After an action the pages on show are asked for again.
#include "mainwindow.h"

#include "../disc/disccontroller.h"
#include "../engine/engineclient.h"
#include "../media/player.h"
#include "../media/vlcplayback.h"
#include "../sdui/pagehost.h"
#include "../sdui/sduiview.h"
#include "controlbar.h"
#include "theme.h"

#include <QApplication>
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFileDialog>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QPointer>
#include <QProcess>

namespace wmp {

namespace {
QString s(const QJsonObject &o, const char *k) { return o.value(QLatin1String(k)).toString(); }
} // namespace

QJsonObject MainWindow::pageState()
{
    QJsonObject st;
    st.insert(QStringLiteral("version"), QCoreApplication::applicationVersion());
    QJsonArray recent;
    for (const QString &p : settings_.value(QStringLiteral("recent")).toStringList())
        recent.append(QJsonObject{{"path", p}, {"gone", !QFileInfo::exists(p)}});
    st.insert(QStringLiteral("recent"), recent);

    const bool useVlc = settings_.value(QStringLiteral("useVlc"), false).toBool();
    QString folder = settings_.value(QStringLiteral("vlcFolder")).toString();
    if (folder.isEmpty() && useVlc)
        folder = VlcPlayback::detectFolder();
    st.insert(QStringLiteral("useVlc"), useVlc);
    st.insert(QStringLiteral("vlcFolder"), folder);
    st.insert(QStringLiteral("vlcStatus"), vlcStatus_);
    st.insert(QStringLiteral("checkModules"), settings_.value(QStringLiteral("checkModules"), false).toBool());
    QJsonObject offers;
    for (auto it = offers_.cbegin(); it != offers_.cend(); ++it)
        offers.insert(it.key(), it.value());
    st.insert(QStringLiteral("offers"), offers);
    st.insert(QStringLiteral("moduleNote"), moduleNote_);
    st.insert(QStringLiteral("notice"), startNotice_);

    st.insert(QStringLiteral("theme"), Themes::current().id);
    QJsonArray themes;
    for (const QString &id : Themes::allIds()) {
        const Theme t = Themes::load(id);
        auto c = [](const QColor &col) { return col.name(QColor::HexRgb); };
        themes.append(QJsonObject{{"id", id}, {"name", t.name}, {"scheme", t.scheme}, {"description", t.description}, {"imported", t.imported},
                                  {"tokens", QJsonObject{{"bg", c(t.bg)}, {"bg2", c(t.bg2)}, {"bg3", c(t.bg3)}, {"bg4", c(t.bg4)}, {"fg", c(t.fg)}, {"accent", c(t.accent)}, {"focus", c(t.focus)}}}});
    }
    st.insert(QStringLiteral("themes"), themes);
    st.insert(QStringLiteral("finishes"), Themes::current().finish.any());
    st.insert(QStringLiteral("finish"), Themes::flat() ? QStringLiteral("flat") : QStringLiteral("theme"));
    st.insert(QStringLiteral("firstRun"), firstRun_);
    st.insert(QStringLiteral("themeNote"), themeNote_);
    st.insert(QStringLiteral("obeyLocks"), settings_.value(QStringLiteral("obeyLocks"), true).toBool());
    st.insert(QStringLiteral("forcedSubs"), settings_.value(QStringLiteral("forcedSubs"), true).toBool());

    QString kind = QStringLiteral("video");
    int angles = 1;
    if (disc_->active()) {
        const DiscGuide &g = disc_->guide();
        kind = disc_->kind().isEmpty() ? g.kind : disc_->kind();
        if (kind != QLatin1String("bluray"))
            kind = QStringLiteral("dvd");
        angles = disc_->angles();
        const DiscPick pick = disc_->pick();
        QString chosen;
        if (pick.valid())
            chosen = pick.id.isEmpty() ? QStringLiteral("title:%1").arg(pick.title) : pick.id;
        QJsonObject d{{"kind", kind}, {"label", disc_->label()}, {"chosen", chosen}, {"playingTitle", canMakeMain() ? disc_->ttn() : 0}};
        if (g.present)
            d.insert(QStringLiteral("guide"), g.raw);
        st.insert(QStringLiteral("disc"), d);
    } else if (cdOn()) {
        kind = QStringLiteral("cd");
    }
    st.insert(QStringLiteral("remote"), QJsonObject{{"kind", kind}, {"angles", angles}, {"page", remotePage_}});
    return st;
}

void MainWindow::reloadPages()
{
    pages_->reload(start_);
    if (dialog_)
        pages_->reload(dialog_->view());
    if (remote_)
        pages_->reload(remote_->view());
}

void MainWindow::showPage(const QString &page, bool firstRun)
{
    firstRun_ = firstRun;
    if (page == QLatin1String("settings") || page == QLatin1String("look") || page == QLatin1String("modules"))
        updateVlcStatus();
    if (dialog_) {
        pages_->show(dialog_->view(), page);
        dialog_->raise();
        dialog_->activateWindow();
        return;
    }
    dialog_ = new PageDialog(pages_, page, this);
    connect(dialog_, &QDialog::finished, this, [this] {
        if (firstRun_) {
            firstRun_ = false;
            settings_.setValue(QStringLiteral("lookChosen"), true);
        }
    });
    dialog_->open();
}

void MainWindow::grabPage(const QString &page, const QString &file)
{
    auto *view = new SduiView;
    view->setAttribute(Qt::WA_DontShowOnScreen);
    view->resize(1000, 760);
    view->setObjectName(QStringLiteral("startPage"));
    view->setAttribute(Qt::WA_StyledBackground);
    updateVlcStatus();
    pages_->show(view, page);
    view->show();
    // The document comes from the helper: wait for it.
    QElapsedTimer t;
    t.start();
    while (view->document().isEmpty() && t.elapsed() < 5000)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 50);
    for (int i = 0; i < 10; i++)
        QCoreApplication::processEvents(QEventLoop::AllEvents, 30);
    view->grab().save(file);
    view->deleteLater();
}

void MainWindow::showRemote()
{
    if (remote_) {
        remote_->raise();
        return;
    }
    remote_ = new PageDialog(pages_, QStringLiteral("remote"), this);
    remote_->setWindowFlag(Qt::Tool);
    remote_->resize(560, 520);
    remote_->show();
    // The keys it dims follow what is open.
    connect(disc_, &DiscController::changed, remote_, [this] {
        if (remote_)
            pages_->reload(remote_->view());
    });
}

void MainWindow::updateVlcStatus()
{
    vlcStatus_.clear();
    if (!settings_.value(QStringLiteral("useVlc"), false).toBool())
        return;
    QString folder = settings_.value(QStringLiteral("vlcFolder")).toString();
    if (folder.isEmpty())
        folder = VlcPlayback::detectFolder();
    QString why;
    if (player_->enableVlc(folder, &why))
        vlcStatus_ = QStringLiteral("Found VLC %1. It takes effect the next time you open something.").arg(player_->vlcVersion());
    else if (folder.isEmpty())
        vlcStatus_ = QStringLiteral("VLC wasn't found on this computer. Install it from videolan.org, or choose its folder.");
    else
        vlcStatus_ = why;
}

void MainWindow::runUpdater(const QStringList &args, const std::function<void(const QJsonObject &)> &done)
{
    auto *proc = new QProcess(this);
    proc->setStandardErrorFile(QProcess::nullDevice());
    connect(proc, &QProcess::finished, this, [proc, done] {
        const QJsonObject o = QJsonDocument::fromJson(proc->readAllStandardOutput()).object();
        proc->deleteLater();
        done(o.isEmpty() ? QJsonObject{{"error", QStringLiteral("the updater stopped unexpectedly")}} : o);
    });
    proc->start(EngineClient::updaterPath(), args, QIODevice::ReadOnly);
}

void MainWindow::handleAction(SduiView *from, const QJsonObject &a)
{
    const QString type = s(a, "type");
    if (type == QLatin1String("navigate")) {
        const QString to = s(a, "to");
        const bool inStart = from == start_;
        if (to == QLatin1String("/")) {
            if (!inStart && dialog_)
                dialog_->close();
            return;
        }
        if (to.startsWith(QLatin1String("/page/"))) {
            const QString page = to.mid(6);
            if (inStart)
                showPage(page);
            else
                pages_->show(from, page);
        }
        return;
    }
    if (type != QLatin1String("command"))
        return; // an action a server might send and this player has no use for: ignored, as the contract says
    const QString op = s(a, "op"), item = s(a, "item");
    const QJsonObject params = a.value(QStringLiteral("params")).toObject();
    const QString path = params.value(QStringLiteral("path")).toString(item);

    if (op == QLatin1String("app.openFile")) {
        openDialog();
    } else if (op == QLatin1String("app.openFolder")) {
        openFolderDialog();
    } else if (op == QLatin1String("app.open")) {
        openPath(path);
    } else if (op == QLatin1String("app.makeImage")) {
        makeImage(path);
    } else if (op == QLatin1String("app.clearRecent")) {
        settings_.remove(QStringLiteral("recent"));
        rebuildRecent();
    } else if (op == QLatin1String("app.vlc.set")) {
        settings_.setValue(QStringLiteral("useVlc"), params.value(QStringLiteral("on")).toBool());
        updateVlcStatus();
        applyEngine();
    } else if (op == QLatin1String("app.vlc.folder")) {
        const QString d = QFileDialog::getExistingDirectory(this, QStringLiteral("Choose VLC's folder"), settings_.value(QStringLiteral("vlcFolder")).toString());
        if (!d.isEmpty()) {
            settings_.setValue(QStringLiteral("vlcFolder"), QDir::toNativeSeparators(d));
            updateVlcStatus();
        }
    } else if (op == QLatin1String("app.modules.check")) {
        moduleNote_ = QStringLiteral("Looking on GitHub…");
        reloadPages();
        runUpdater({QStringLiteral("--check")}, [this](const QJsonObject &res) {
            offers_.clear();
            QStringList notes;
            if (!s(res, "error").isEmpty())
                notes << s(res, "error");
            for (const auto &v : res.value("offers").toArray()) {
                const QJsonObject o = v.toObject();
                if (!s(o, "version").isEmpty())
                    offers_.insert(s(o, "id"), s(o, "version"));
                if (!s(o, "note").isEmpty())
                    notes << s(o, "note");
            }
            if (notes.isEmpty())
                notes << (offers_.isEmpty() ? QStringLiteral("Everything is up to date.") : QStringLiteral("There are updates to install."));
            moduleNote_ = notes.join(QLatin1Char(' '));
            reloadPages();
        });
        return;
    } else if (op == QLatin1String("app.modules.install")) {
        const QString version = params.value(QStringLiteral("version")).toString();
        moduleNote_ = QStringLiteral("Installing %1 %2…").arg(item, version);
        reloadPages();
        runUpdater({QStringLiteral("--install"), item, version}, [this, item, version](const QJsonObject &res) {
            const QString err = s(res, "error");
            moduleNote_ = err.isEmpty() ? QStringLiteral("%1 %2 is installed, and takes over the next time you open a disc.").arg(item, version)
                                        : QStringLiteral("%1 wasn't updated: %2").arg(item, err);
            if (err.isEmpty())
                offers_.remove(item);
            reloadPages();
        });
        return;
    } else if (op == QLatin1String("app.modules.back")) {
        engine_->call(QStringLiteral("module-rollback"), {{"id", item}}, [this, item](const EngineReply &rb) {
            moduleNote_ = rb.ok() ? QStringLiteral("%1 is back to %2, from the next disc you open.").arg(item, rb.result.toObject().value("inUse").toString()) : rb.error;
            reloadPages();
        });
        return;
    } else if (op == QLatin1String("app.modules.atStart")) {
        settings_.setValue(QStringLiteral("checkModules"), params.value(QStringLiteral("on")).toBool());
    } else if (op == QLatin1String("app.theme.set")) {
        chooseTheme(item);
    } else if (op == QLatin1String("app.theme.import")) {
        const QString zip = QFileDialog::getOpenFileName(this, QStringLiteral("Import a theme"), settings_.value(QStringLiteral("lastDir")).toString(),
                                                         QStringLiteral("Theme ZIP (*.zip)"));
        if (!zip.isEmpty()) {
            const ThemeImport r = Themes::importZip(zip);
            if (r.ok) {
                themeNote_ = QStringLiteral("%1 is imported and in use.%2").arg(r.name, r.notes.isEmpty() ? QString() : QLatin1Char(' ') + r.notes.join(QLatin1Char(' ')));
                chooseTheme(r.id);
            } else {
                themeNote_ = QStringLiteral("That theme wasn't imported. %1").arg(r.error);
            }
        }
    } else if (op == QLatin1String("app.theme.remove")) {
        const bool inUse = Themes::current().id == item;
        if (Themes::removeImported(item)) {
            themeNote_ = QStringLiteral("Removed.");
            if (inUse)
                chooseTheme(QStringLiteral("default"));
        }
    } else if (op == QLatin1String("app.finish.set")) {
        Themes::setFlat(item == QLatin1String("flat"));
        settings_.setValue(QStringLiteral("flatLook"), Themes::flat());
        Themes::apply(qApp, Themes::current());
    } else if (op == QLatin1String("app.firstRun.done")) {
        settings_.setValue(QStringLiteral("lookChosen"), true);
        firstRun_ = false;
        if (dialog_)
            dialog_->close();
    } else if (op == QLatin1String("app.disc.choose")) {
        if (disc_->active())
            disc_->choose(item);
        if (dialog_)
            dialog_->close();
        return;
    } else if (op == QLatin1String("app.disc.makeMain")) {
        makeThisTheFeature();
    } else if (op == QLatin1String("app.disc.forget")) {
        forgetChosenFeature();
    } else if (op == QLatin1String("app.disc.locks")) {
        actLocks_->setChecked(params.value(QStringLiteral("on")).toBool());
    } else if (op == QLatin1String("app.disc.forced")) {
        actForced_->setChecked(params.value(QStringLiteral("on")).toBool());
    } else if (op == QLatin1String("app.remote.page")) {
        remotePage_ = params.value(QStringLiteral("page")).toInt();
    } else if (op == QLatin1String("app.remote.key")) {
        pressRemoteKey(item);
        return;
    }
    reloadPages();
}

void MainWindow::chooseTheme(const QString &id)
{
    Themes::apply(qApp, Themes::load(id));
    settings_.setValue(QStringLiteral("theme"), Themes::current().id);
    settings_.setValue(QStringLiteral("lookChosen"), true);
    update();
}

// A key of WittyRemote does what the same key does on the keyboard or in the controls.
void MainWindow::pressRemoteKey(const QString &key)
{
    static const QMap<QString, int> keys = {{"up", Qt::Key_Up}, {"down", Qt::Key_Down}, {"left", Qt::Key_Left}, {"right", Qt::Key_Right},
                                            {"enter", Qt::Key_Return}, {"previous", Qt::Key_P}, {"next", Qt::Key_N}, {"return", Qt::Key_Backspace}};
    if (key == QLatin1String("topMenu")) {
        disc_->titleMenu();
    } else if (key == QLatin1String("menu")) {
        disc_->rootMenu();
    } else if (key == QLatin1String("angle")) {
        if (disc_->active() && disc_->angles() > 1)
            disc_->switchAngle(disc_->angle() % disc_->angles() + 1);
    } else if (key == QLatin1String("audio")) {
        cycleAudio();
    } else if (key == QLatin1String("subtitle")) {
        cycleSubtitle();
    } else if (key == QLatin1String("play") || key == QLatin1String("pause")) {
        togglePlay();
    } else if (key == QLatin1String("stop") || key == QLatin1String("eject")) {
        stopPlayback();
    } else if (key == QLatin1String("back") || key == QLatin1String("forward")) {
        const double d = key == QLatin1String("forward") ? 30 : -10;
        if (disc_->active())
            disc_->seekBy(d);
        else if (player_->hasMedia())
            player_->seek(player_->position() + d);
    } else if (key.size() == 1 && key[0].isDigit()) {
        handleKey(Qt::Key_0 + key[0].digitValue());
    } else if (keys.contains(key)) {
        handleKey(keys.value(key));
    } else {
        setStatus(QStringLiteral("This player can't send the %1 key to a disc.").arg(key));
    }
}

} // namespace wmp

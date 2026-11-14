CREATE TABLE "Users" (
    "Id" TEXT NOT NULL CONSTRAINT "PK_Users" PRIMARY KEY,
    "AudioLanguagePreference" TEXT NULL,
    "AuthenticationProviderId" TEXT NOT NULL,
    "CastReceiverId" TEXT NULL,
    "DisplayCollectionsView" INTEGER NOT NULL,
    "DisplayMissingEpisodes" INTEGER NOT NULL,
    "EnableAutoLogin" INTEGER NOT NULL,
    "EnableLocalPassword" INTEGER NOT NULL,
    "EnableNextEpisodeAutoPlay" INTEGER NOT NULL,
    "EnableUserPreferenceAccess" INTEGER NOT NULL,
    "HidePlayedInLatest" INTEGER NOT NULL,
    "InternalId" INTEGER NOT NULL,
    "InvalidLoginAttemptCount" INTEGER NOT NULL,
    "LastActivityDate" TEXT NULL,
    "LastLoginDate" TEXT NULL,
    "LoginAttemptsBeforeLockout" INTEGER NULL,
    "MaxActiveSessions" INTEGER NOT NULL,
    "MaxParentalAgeRating" INTEGER NULL,
    "MustUpdatePassword" INTEGER NOT NULL,
    "Password" TEXT NULL,
    "PasswordResetProviderId" TEXT NOT NULL,
    "PlayDefaultAudioTrack" INTEGER NOT NULL,
    "RememberAudioSelections" INTEGER NOT NULL,
    "RememberSubtitleSelections" INTEGER NOT NULL,
    "RemoteClientBitrateLimit" INTEGER NULL,
    "RowVersion" INTEGER NOT NULL,
    "SubtitleLanguagePreference" TEXT NULL,
    "SubtitleMode" INTEGER NOT NULL,
    "SyncPlayAccess" INTEGER NOT NULL,
    "Username" TEXT NOT NULL
);
INSERT INTO "Users" ("Id","AudioLanguagePreference","AuthenticationProviderId","CastReceiverId","DisplayCollectionsView","DisplayMissingEpisodes","EnableAutoLogin","EnableLocalPassword","EnableNextEpisodeAutoPlay","EnableUserPreferenceAccess","HidePlayedInLatest","InternalId","InvalidLoginAttemptCount","LastActivityDate","LastLoginDate","LoginAttemptsBeforeLockout","MaxActiveSessions","MustUpdatePassword","Password","PasswordResetProviderId","PlayDefaultAudioTrack","RememberAudioSelections","RememberSubtitleSelections","RemoteClientBitrateLimit","RowVersion","SubtitleLanguagePreference","SubtitleMode","SyncPlayAccess","Username") VALUES ('11111111-1111-4111-8111-111111111111', 'eng', 'Jellyfin.Server.Implementations.Users.DefaultAuthenticationProvider', 'F007D354', 0, 0, 0, 0, 1, 1, 1, 1, 0, '2024-05-02 09:30:00.1234567', '2024-05-01 20:00:00', NULL, 0, 0, '$PBKDF2-SHA512$iterations=210000$000102030405060708090A0B0C0D0E0F$EC76EF2A24A57C3B742DD5F10B0712C497639F89B3578FFB5F2E132CF9705D952868506851B9A897A6737543BF6AC5FBA27BC3233D1888D0A68F3A8B5655CA50', 'Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider', 1, 1, 1, NULL, 3, 'eng', 4, 0, 'alice');
INSERT INTO "Users" ("Id","AudioLanguagePreference","AuthenticationProviderId","CastReceiverId","DisplayCollectionsView","DisplayMissingEpisodes","EnableAutoLogin","EnableLocalPassword","EnableNextEpisodeAutoPlay","EnableUserPreferenceAccess","HidePlayedInLatest","InternalId","InvalidLoginAttemptCount","LastActivityDate","LastLoginDate","LoginAttemptsBeforeLockout","MaxActiveSessions","MustUpdatePassword","Password","PasswordResetProviderId","PlayDefaultAudioTrack","RememberAudioSelections","RememberSubtitleSelections","RemoteClientBitrateLimit","RowVersion","SubtitleLanguagePreference","SubtitleMode","SyncPlayAccess","Username") VALUES ('22222222-2222-4222-8222-222222222222', NULL, 'Jellyfin.Server.Implementations.Users.DefaultAuthenticationProvider', NULL, 0, 1, 0, 0, 0, 1, 0, 2, 0, NULL, NULL, 5, 2, 0, NULL, 'Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider', 0, 1, 1, 8000000, 1, NULL, 1, 1, 'bob');
INSERT INTO "Users" ("Id","AudioLanguagePreference","AuthenticationProviderId","CastReceiverId","DisplayCollectionsView","DisplayMissingEpisodes","EnableAutoLogin","EnableLocalPassword","EnableNextEpisodeAutoPlay","EnableUserPreferenceAccess","HidePlayedInLatest","InternalId","InvalidLoginAttemptCount","LastActivityDate","LastLoginDate","LoginAttemptsBeforeLockout","MaxActiveSessions","MustUpdatePassword","Password","PasswordResetProviderId","PlayDefaultAudioTrack","RememberAudioSelections","RememberSubtitleSelections","RemoteClientBitrateLimit","RowVersion","SubtitleLanguagePreference","SubtitleMode","SyncPlayAccess","Username") VALUES ('33333333-3333-4333-8333-333333333333', NULL, 'Jellyfin.Server.Implementations.Users.DefaultAuthenticationProvider', NULL, 0, 0, 0, 0, 1, 1, 1, 3, 0, NULL, NULL, NULL, 0, 0, '$PBKDF2-SHA512$iterations=210000$101112131415161718191A1B1C1D1E1F$7483480C07EFC637058CD08588FF831972AEDCD0B4778FB366105A322B76B99A505F71A889480BBDC5A4523E7B4DA4F23EBBB74A7FD04C978810D0889410CC6C', 'Jellyfin.Server.Implementations.Users.DefaultPasswordResetProvider', 1, 1, 1, NULL, 1, NULL, 0, 0, 'Carol');

CREATE TABLE "Permissions" (
    "Id" INTEGER NOT NULL CONSTRAINT "PK_Permissions" PRIMARY KEY AUTOINCREMENT,
    "Kind" INTEGER NOT NULL,
    "Permission_Permissions_Guid" TEXT NULL,
    "RowVersion" INTEGER NOT NULL,
    "UserId" TEXT NULL,
    "Value" INTEGER NOT NULL
);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (0, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (1, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (2, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (3, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (4, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (5, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (6, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (7, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (8, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (9, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (10, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (11, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (12, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (13, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (14, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (15, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (16, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (17, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (18, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (19, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (20, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (21, 0, '11111111-1111-4111-8111-111111111111', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (22, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (23, 0, '11111111-1111-4111-8111-111111111111', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (0, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (1, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (2, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (3, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (4, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (5, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (6, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (7, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (8, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (9, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (10, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (11, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (12, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (13, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (14, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (15, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (16, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (17, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (18, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (19, 0, '22222222-2222-4222-8222-222222222222', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (20, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (21, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (22, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (23, 0, '22222222-2222-4222-8222-222222222222', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (0, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (1, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (2, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (3, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (4, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (5, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (6, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (7, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (8, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (9, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (10, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (11, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (12, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (13, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (14, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (15, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (16, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (17, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (18, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (19, 0, '33333333-3333-4333-8333-333333333333', 1);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (20, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (21, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (22, 0, '33333333-3333-4333-8333-333333333333', 0);
INSERT INTO "Permissions" ("Kind","RowVersion","UserId","Value") VALUES (23, 0, '33333333-3333-4333-8333-333333333333', 0);

CREATE TABLE "Devices" (
    "Id" INTEGER NOT NULL CONSTRAINT "PK_Devices" PRIMARY KEY AUTOINCREMENT,
    "AccessToken" TEXT NOT NULL,
    "AppName" TEXT NOT NULL,
    "AppVersion" TEXT NOT NULL,
    "DateCreated" TEXT NOT NULL,
    "DateLastActivity" TEXT NOT NULL,
    "DateModified" TEXT NOT NULL,
    "DeviceId" TEXT NOT NULL,
    "DeviceName" TEXT NOT NULL,
    "IsActive" INTEGER NOT NULL,
    "UserId" TEXT NOT NULL
);
INSERT INTO "Devices" ("AccessToken","AppName","AppVersion","DateCreated","DateLastActivity","DateModified","DeviceId","DeviceName","IsActive","UserId") VALUES ('a11ce0000000000000000000000000a1', 'Jellyfin Web', '10.10.0', '2023-01-01 00:00:00', '2024-05-02 09:30:00.1234567', '2024-05-02 09:30:00.1234567', 'device-alice-web', 'Chrome', 1, '11111111-1111-4111-8111-111111111111');
INSERT INTO "Devices" ("AccessToken","AppName","AppVersion","DateCreated","DateLastActivity","DateModified","DeviceId","DeviceName","IsActive","UserId") VALUES ('a11ce0000000000000000000000000a2', 'Jellyfin Android TV', '0.18.0', '2023-01-01 00:00:00', '2024-04-20 21:00:00', '2024-04-20 21:00:00', 'device-living-room', 'Living Room TV', 1, '11111111-1111-4111-8111-111111111111');
INSERT INTO "Devices" ("AccessToken","AppName","AppVersion","DateCreated","DateLastActivity","DateModified","DeviceId","DeviceName","IsActive","UserId") VALUES ('b0b00000000000000000000000000b01', 'Jellyfin Android TV', '0.18.0', '2023-01-01 00:00:00', '2024-04-28 19:00:00', '2024-04-28 19:00:00', 'device-living-room', 'Living Room TV', 1, '22222222-2222-4222-8222-222222222222');
INSERT INTO "Devices" ("AccessToken","AppName","AppVersion","DateCreated","DateLastActivity","DateModified","DeviceId","DeviceName","IsActive","UserId") VALUES ('ca401000000000000000000000000c01', 'Jellyfin Web', '10.10.0', '2023-01-01 00:00:00', '2024-04-01 10:00:00', '2024-04-01 10:00:00', 'device-carol-web', 'Firefox', 1, '33333333-3333-4333-8333-333333333333');
INSERT INTO "Devices" ("AccessToken","AppName","AppVersion","DateCreated","DateLastActivity","DateModified","DeviceId","DeviceName","IsActive","UserId") VALUES ('a11ce0000000000000000000000000a9', 'Jellyfin Web', '10.9.0', '2023-01-01 00:00:00', '2023-01-01 00:00:00', '2023-01-01 00:00:00', 'device-alice-old', 'Chrome', 0, '11111111-1111-4111-8111-111111111111');

CREATE TABLE "ApiKeys" (
    "Id" INTEGER NOT NULL CONSTRAINT "PK_ApiKeys" PRIMARY KEY AUTOINCREMENT,
    "AccessToken" TEXT NOT NULL,
    "DateCreated" TEXT NOT NULL,
    "DateLastActivity" TEXT NOT NULL,
    "Name" TEXT NOT NULL
);
INSERT INTO "ApiKeys" ("AccessToken","DateCreated","DateLastActivity","Name") VALUES ('ap1ke70000000000000000000000f001', '2023-06-01 12:00:00', '2024-05-01 00:00:00', 'Homepage');
